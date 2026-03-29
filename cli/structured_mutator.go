package cli

import (
	"bytes"
	"os"
)

func MutateStructuredFile[T any](
	path string,
	load func([]byte) (*T, error),
	zero func() *T,
	mutate func(*T) error,
	save func(*T) ([]byte, error),
) error {
	return mutateStructuredFile(path, 0o644, load, zero, mutate, save)
}

func mutateStructuredFile[T any](
	path string,
	defaultMode os.FileMode,
	load func([]byte) (*T, error),
	zero func() *T,
	mutate func(*T) error,
	save func(*T) ([]byte, error),
) error {
	return withExclusiveFileLock(path, func() error {
		state, mode, err := loadStructuredFileState(path, defaultMode, load, zero)
		if err != nil {
			return err
		}
		if err := mutate(state); err != nil {
			return err
		}
		data, err := save(state)
		if err != nil {
			return err
		}
		return writeFileAtomic(path, data, mode)
	})
}

func loadStructuredFileState[T any](
	path string,
	defaultMode os.FileMode,
	load func([]byte) (*T, error),
	zero func() *T,
) (*T, os.FileMode, error) {
	mode := defaultMode
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return zero(), mode, nil
		}
		return nil, 0, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return zero(), mode, nil
	}
	state, err := load(data)
	if err != nil {
		return nil, 0, err
	}
	if state == nil {
		return zero(), mode, nil
	}
	return state, mode, nil
}
