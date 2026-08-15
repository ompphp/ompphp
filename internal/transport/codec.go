package transport

import (
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/KarpelesLab/goro/core/phpv"
	publicasync "github.com/ompphp/ompphp/async"
)

const (
	DefaultMaxDepth = 32
	DefaultMaxBytes = 1 << 20
)

var (
	ErrCycle       = errors.New("transfer value contains a cycle")
	ErrTooDeep     = errors.New("transfer value exceeds the nesting limit")
	ErrTooLarge    = errors.New("transfer value exceeds the size limit")
	ErrUnsupported = errors.New("value cannot cross a runtime boundary")
)

type Key = publicasync.Key
type Entry = publicasync.Entry
type Map = publicasync.Map

type Limits struct {
	MaxDepth int
	MaxBytes int
}

func DefaultLimits() Limits { return Limits{MaxDepth: DefaultMaxDepth, MaxBytes: DefaultMaxBytes} }

func FromPHP(ctx phpv.Context, value *phpv.ZVal, limits Limits) (any, error) {
	state := codecState{limits: normalize(limits), arrays: make(map[*phpv.ZHashTable]bool)}
	return state.fromPHP(ctx, value, 0)
}

func ToPHP(value any, limits Limits) (*phpv.ZVal, error) {
	state := codecState{limits: normalize(limits), containers: make(map[uintptr]bool)}
	return state.toPHP(value, 0)
}

type codecState struct {
	limits     Limits
	bytes      int
	arrays     map[*phpv.ZHashTable]bool
	containers map[uintptr]bool
}

func normalize(limits Limits) Limits {
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = DefaultMaxDepth
	}
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = DefaultMaxBytes
	}
	return limits
}

func (s *codecState) add(size int) error {
	s.bytes += size
	if s.bytes > s.limits.MaxBytes {
		return ErrTooLarge
	}
	return nil
}

func (s *codecState) addEntries(count int) error {
	if count < 0 || count > (s.limits.MaxBytes-s.bytes)/16 {
		return ErrTooLarge
	}
	return s.add(count * 16)
}

func (s *codecState) enter(value any) (func(), error) {
	pointer := reflect.ValueOf(value).Pointer()
	if pointer == 0 {
		return func() {}, nil
	}
	if s.containers[pointer] {
		return nil, ErrCycle
	}
	s.containers[pointer] = true
	return func() { delete(s.containers, pointer) }, nil
}

func (s *codecState) fromPHP(ctx phpv.Context, value *phpv.ZVal, depth int) (any, error) {
	if depth > s.limits.MaxDepth {
		return nil, ErrTooDeep
	}
	if value == nil {
		return nil, nil
	}
	switch value.GetType() {
	case phpv.ZtNull:
		return nil, s.add(1)
	case phpv.ZtBool:
		return bool(value.AsBool(ctx)), s.add(1)
	case phpv.ZtInt:
		return int64(value.AsInt(ctx)), s.add(8)
	case phpv.ZtFloat:
		return float64(value.AsFloat(ctx)), s.add(8)
	case phpv.ZtString:
		text := string(value.AsString(ctx))
		return text, s.add(len(text))
	case phpv.ZtArray:
		array := value.AsArray(ctx)
		if s.arrays[array.H()] {
			return nil, ErrCycle
		}
		s.arrays[array.H()] = true
		defer delete(s.arrays, array.H())
		count := int(array.Count(ctx))
		if err := s.addEntries(count); err != nil {
			return nil, err
		}
		result := make(Map, 0, count)
		for key, item := range array.Iterate(ctx) {
			entry := Entry{}
			switch key.GetType() {
			case phpv.ZtInt:
				entry.Key = Key{Integer: int64(key.AsInt(ctx)), IsInt: true}
				if err := s.add(8); err != nil {
					return nil, err
				}
			case phpv.ZtString:
				entry.Key.String = string(key.AsString(ctx))
				if err := s.add(len(entry.Key.String)); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("%w: array key type %s", ErrUnsupported, key.GetType().TypeName())
			}
			converted, err := s.fromPHP(ctx, item, depth+1)
			if err != nil {
				return nil, err
			}
			entry.Value = converted
			result = append(result, entry)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%w: PHP %s", ErrUnsupported, value.GetType().TypeName())
	}
}

func (s *codecState) toPHP(value any, depth int) (*phpv.ZVal, error) {
	if depth > s.limits.MaxDepth {
		return nil, ErrTooDeep
	}
	switch value := value.(type) {
	case nil:
		return phpv.ZNULL.ZVal(), s.add(1)
	case bool:
		return phpv.ZBool(value).ZVal(), s.add(1)
	case int:
		return phpv.ZInt(value).ZVal(), s.add(8)
	case int32:
		return phpv.ZInt(value).ZVal(), s.add(8)
	case int64:
		return phpv.ZInt(value).ZVal(), s.add(8)
	case float32:
		return phpv.ZFloat(value).ZVal(), s.add(8)
	case float64:
		return phpv.ZFloat(value).ZVal(), s.add(8)
	case string:
		return phpv.ZString(value).ZVal(), s.add(len(value))
	case []any:
		leave, err := s.enter(value)
		if err != nil {
			return nil, err
		}
		defer leave()
		if err := s.addEntries(len(value)); err != nil {
			return nil, err
		}
		array := phpv.NewZArray()
		for _, item := range value {
			converted, err := s.toPHP(item, depth+1)
			if err != nil {
				return nil, err
			}
			_ = array.OffsetSet(nil, nil, converted)
		}
		return array.ZVal(), nil
	case map[string]any:
		leave, err := s.enter(value)
		if err != nil {
			return nil, err
		}
		defer leave()
		if err := s.addEntries(len(value)); err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		array := phpv.NewZArray()
		for _, key := range keys {
			converted, err := s.toPHP(value[key], depth+1)
			if err != nil {
				return nil, err
			}
			if err := s.add(len(key)); err != nil {
				return nil, err
			}
			_ = array.OffsetSet(nil, phpv.ZString(key).ZVal(), converted)
		}
		return array.ZVal(), nil
	case Map:
		leave, err := s.enter(value)
		if err != nil {
			return nil, err
		}
		defer leave()
		if err := s.addEntries(len(value)); err != nil {
			return nil, err
		}
		array := phpv.NewZArray()
		for _, entry := range value {
			converted, err := s.toPHP(entry.Value, depth+1)
			if err != nil {
				return nil, err
			}
			var key *phpv.ZVal
			if entry.Key.IsInt {
				key = phpv.ZInt(entry.Key.Integer).ZVal()
				if err := s.add(8); err != nil {
					return nil, err
				}
			} else {
				key = phpv.ZString(entry.Key.String).ZVal()
				if err := s.add(len(entry.Key.String)); err != nil {
					return nil, err
				}
			}
			_ = array.OffsetSet(nil, key, converted)
		}
		return array.ZVal(), nil
	default:
		return nil, fmt.Errorf("%w: Go %T", ErrUnsupported, value)
	}
}
