package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSeparateWorkerProcessesRemainIsolatedUnderLoad(t *testing.T) {
	bootstrap := filepath.Join(t.TempDir(), "autoload.php")
	code := `<?php
final class IsolatedCounterTask {
    public function __invoke(mixed $unused): int {
        $GLOBALS['worker_counter'] = ($GLOBALS['worker_counter'] ?? 0) + 1;
        return $GLOBALS['worker_counter'];
    }
}`
	if err := os.WriteFile(bootstrap, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}

	const workers = 8
	const calls = 250
	instances := make([]*phpWorker, 0, workers)
	for id := 1; id <= workers; id++ {
		worker, err := newPHPWorker(context.Background(), id, bootstrap)
		if err != nil {
			t.Fatal(err)
		}
		instances = append(instances, worker)
	}
	t.Cleanup(func() {
		for _, worker := range instances {
			worker.Close()
		}
	})

	var group sync.WaitGroup
	errors := make(chan error, workers)
	for _, worker := range instances {
		worker := worker
		group.Add(1)
		go func() {
			defer group.Done()
			for expected := int64(1); expected <= calls; expected++ {
				value, remote := worker.Run("IsolatedCounterTask", nil)
				if remote != nil {
					errors <- remote
					return
				}
				if value != expected {
					errors <- fmt.Errorf("worker %d counter = %v, want %d", worker.id, value, expected)
					return
				}
			}
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
