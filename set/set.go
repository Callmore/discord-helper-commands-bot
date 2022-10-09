package set

import (
	"encoding/json"
	"fmt"
)

type Set[T comparable] struct {
	m map[T]struct{}
}

func (s *Set[T]) Add(v T) {
	if s.m == nil {
		s.m = make(map[T]struct{})
	}
	s.m[v] = struct{}{}
}

func (s *Set[T]) Remove(v T) {
	delete(s.m, v)
}

func (s *Set[T]) Has(v T) bool {
	_, ok := s.m[v]
	return ok
}

func (s *Set[T]) Len() int {
	return len(s.m)
}

func (s *Set[T]) Clear() {
	s.m = nil
}

func (s *Set[T]) Values() []T {
	if s.m == nil {
		return nil
	}
	values := make([]T, 0, len(s.m))
	for v := range s.m {
		values = append(values, v)
	}
	return values
}

func (s *Set[T]) String() string {
	return fmt.Sprint(s.Values())
}

func (s Set[T]) MarshalJSON() ([]byte, error) {
	out := map[T]bool{}
	for k := range s.m {
		out[k] = true
	}
	return json.Marshal(out)
}

func (s *Set[T]) UnmarshalJSON(data []byte) error {
	out := map[T]bool{}
	err := json.Unmarshal(data, &out)
	if err != nil {
		return err
	}
	for k := range out {
		s.Add(k)
	}
	return nil
}
