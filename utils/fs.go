package utils

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jxsl13/module-migration/defaults"
)

func Exists(filePath string) (fs.FileInfo, bool, error) {
	fi, err := os.Lstat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return fi, true, nil
}

func Copy(ctx context.Context, src, targetDir string) error {
	src = filepath.Clean(src)

	targetDir = filepath.Clean(targetDir)
	fi, found, err := Exists(targetDir)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("additional file or directory target directory %s could not be found: %w", targetDir, err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("target directory %s is not a directory", targetDir)
	}

	fi, found, err = Exists(src)
	if err != nil {
		return err
	}

	if !found {
		return fmt.Errorf("additional file or directory %s could not be found: %w", src, err)
	}

	if fi.IsDir() {
		_, err = ExecuteQuietPathApplicationWithOutput(ctx, "", "cp", "-Rf", src, targetDir+defaults.FilePathSeparator)
		if err != nil {
			return fmt.Errorf("failed to copy directory %s into %s: %w", src, targetDir, err)
		}
	} else {
		_, err = ExecuteQuietPathApplicationWithOutput(ctx, "", "cp", "-f", src, targetDir+defaults.FilePathSeparator)
		if err != nil {
			return fmt.Errorf("failed to copy file %s into %s: %w", src, targetDir, err)
		}
	}
	return nil
}
