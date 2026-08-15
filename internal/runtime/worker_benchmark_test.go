package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	oconcurrency "github.com/ompphp/ompphp/internal/concurrency"
	"github.com/ompphp/ompphp/internal/transport"
)

func benchmarkBootstrap(b *testing.B) string {
	b.Helper()
	bootstrap := filepath.Join(b.TempDir(), "autoload.php")
	code := `<?php
final class BenchmarkTask { public function __invoke(mixed $value): int { return $value + 1; } }
final class BenchmarkActor {
    public function __construct(private int $value) {}
    public function add(mixed $value): int { return $this->value += $value; }
}
final class StreamerBenchmarkActor {
    public function __construct(private array $entities) {}
    public function playerMoved(mixed $players): array {
        $visible = [];
        foreach ($players as $player) {
            $count = 0;
            foreach ($this->entities as $entity) {
                if ($entity['world'] === $player['world'] && abs($entity['x'] - $player['x']) < 100) $count++;
            }
            $visible[] = $count;
        }
        return $visible;
    }
}`
	if err := os.WriteFile(bootstrap, []byte(code), 0o600); err != nil {
		b.Fatal(err)
	}
	return bootstrap
}

func benchmarkWorker(b *testing.B) *phpWorker {
	b.Helper()
	bootstrap := benchmarkBootstrap(b)
	worker, err := newPHPWorker(context.Background(), 1, bootstrap)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(worker.Close)
	return worker
}

func BenchmarkPHPWorkerRoundTrip(b *testing.B) {
	worker := benchmarkWorker(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, remote := worker.Run("BenchmarkTask", int64(i)); remote != nil {
			b.Fatal(remote)
		}
	}
}

func BenchmarkPersistentActorCall(b *testing.B) {
	worker := benchmarkWorker(b)
	if remote := worker.SpawnActor(1, "BenchmarkActor", int64(0)); remote != nil {
		b.Fatal(remote)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, remote := worker.CallActor(1, "add", int64(1)); remote != nil {
			b.Fatal(remote)
		}
	}
}

func BenchmarkStreamerActor(b *testing.B) {
	worker := benchmarkWorker(b)
	worker.limits = transport.Limits{MaxDepth: 32, MaxBytes: 64 << 20}
	entities := make(transport.Map, 10_000)
	for index := range entities {
		entities[index] = transport.Entry{Key: transport.Key{Integer: int64(index), IsInt: true}, Value: transport.Map{
			{Key: transport.Key{String: "x"}, Value: float64(index % 1000)},
			{Key: transport.Key{String: "world"}, Value: int64(index % 4)},
		}}
	}
	if remote := worker.SpawnActor(1, "StreamerBenchmarkActor", entities); remote != nil {
		b.Fatal(remote)
	}
	for _, players := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("%d-players", players), func(b *testing.B) {
			payload := make(transport.Map, players)
			for index := range payload {
				payload[index] = transport.Entry{Key: transport.Key{Integer: int64(index), IsInt: true}, Value: transport.Map{
					{Key: transport.Key{String: "x"}, Value: float64(index % 1000)},
					{Key: transport.Key{String: "world"}, Value: int64(index % 4)},
				}}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, remote := worker.CallActor(1, "playerMoved", payload); remote != nil {
					b.Fatal(remote)
				}
			}
		})
	}
}

func BenchmarkStreamerActorPool(b *testing.B) {
	bootstrap := benchmarkBootstrap(b)
	scheduler, err := oconcurrency.New(oconcurrency.Config{Workers: 4, TaskQueue: 64, NativeWorkers: 2, NativeQueue: 64, CompletionQueue: 64, ActorMailbox: 16}, func(id int) (oconcurrency.Runner, error) {
		worker, err := newPHPWorker(context.Background(), id, bootstrap)
		if worker != nil {
			worker.limits = transport.Limits{MaxDepth: 32, MaxBytes: 64 << 20}
		}
		return worker, err
	})
	if err != nil {
		b.Fatal(err)
	}
	defer scheduler.Close()
	entities := make(transport.Map, 10_000)
	for index := range entities {
		entities[index] = transport.Entry{Key: transport.Key{Integer: int64(index), IsInt: true}, Value: transport.Map{
			{Key: transport.Key{String: "x"}, Value: float64(index % 1000)}, {Key: transport.Key{String: "world"}, Value: int64(index % 4)},
		}}
	}
	_, actors, err := scheduler.SpawnActorPool("StreamerBenchmarkActor", 4, entities)
	if err != nil {
		b.Fatal(err)
	}
	for range actors {
		for {
			completions := scheduler.Drain(1)
			if len(completions) == 1 {
				scheduler.Acknowledge(completions[0].ID)
				break
			}
		}
	}
	for _, players := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("%d-players", players), func(b *testing.B) {
			shards := make([]transport.Map, len(actors))
			for index := range players {
				shard := index % len(actors)
				shards[shard] = append(shards[shard], transport.Entry{Key: transport.Key{Integer: int64(index), IsInt: true}, Value: transport.Map{{Key: transport.Key{String: "x"}, Value: float64(index % 1000)}, {Key: transport.Key{String: "world"}, Value: int64(index % 4)}}})
			}
			b.ResetTimer()
			for b.Loop() {
				for shard, actor := range actors {
					if _, err := scheduler.CallActor(actor.ActorID, "playerMoved", shards[shard]); err != nil {
						b.Fatal(err)
					}
				}
				completed := 0
				for completed < len(actors) {
					completions := scheduler.Drain(len(actors) - completed)
					for _, completion := range completions {
						scheduler.Acknowledge(completion.ID)
					}
					completed += len(completions)
				}
			}
		})
	}
}
