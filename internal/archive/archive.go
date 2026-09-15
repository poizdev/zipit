// Package archive creates ZIP archives from directory trees.
package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Request describes one archive operation.
type Request struct {
	Source  string
	Output  string
	Matcher Matcher
	Force   bool
	DryRun  bool
}

// Stats describes entries actually encountered during traversal. Files below
// pruned directories are intentionally not enumerated or counted.
type Stats struct {
	FilesIncluded       int64
	FilesExcluded       int64
	DirectoriesExcluded int64
	BytesIncluded       int64
	SymlinksIncluded    int64
	SpecialFilesSkipped int64
}

// Result describes a completed archive operation or dry-run.
type Result struct {
	Stats       Stats
	ArchiveSize int64
}

// Matcher selects root-relative filesystem entries for exclusion.
type Matcher interface {
	Match(path string, isDir bool) bool
}

// ResolveRequest validates source and determines an absolute output path.
func ResolveRequest(source, output string) (Request, error) {
	if source == "" {
		source = "."
	}

	absSource, err := filepath.Abs(source)
	if err != nil {
		return Request{}, fmt.Errorf("resolve source %q: %w", source, err)
	}
	absSource = filepath.Clean(absSource)
	resolvedSource, err := filepath.EvalSymlinks(absSource)
	if err != nil {
		return Request{}, fmt.Errorf("resolve source %q: %w", source, err)
	}
	absSource = filepath.Clean(resolvedSource)
	info, err := os.Stat(absSource)
	if err != nil {
		return Request{}, fmt.Errorf("inspect source %q: %w", source, err)
	}
	if !info.IsDir() {
		return Request{}, fmt.Errorf("source %q is not a directory", source)
	}

	if output == "" {
		output = absSource + ".zip"
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return Request{}, fmt.Errorf("resolve output %q: %w", output, err)
	}
	absOutput = filepath.Clean(absOutput)
	resolvedOutputDir, err := filepath.EvalSymlinks(filepath.Dir(absOutput))
	if err == nil {
		absOutput = filepath.Join(resolvedOutputDir, filepath.Base(absOutput))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Request{}, fmt.Errorf("resolve output directory %q: %w", filepath.Dir(output), err)
	}

	return Request{
		Source: absSource,
		Output: filepath.Clean(absOutput),
	}, nil
}

// Create writes a complete ZIP to a temporary file and publishes it only after
// every entry and writer has closed successfully. Symlinks are preserved as
// link entries without following their targets; unsupported special files are
// skipped and never opened as regular files.
func Create(request Request) (err error) {
	_, err = Run(context.Background(), request)
	return err
}

func create(request Request, publish func(string, string) error) (err error) {
	_, err = run(context.Background(), request, publish)
	return err
}

// Run traverses a source tree once, either writing a ZIP or collecting dry-run
// statistics. It is cancellable through ctx.
func Run(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	publish := publishNoReplace
	if request.Force {
		publish = publishReplace
	}
	return run(ctx, request, publish)
}

func run(ctx context.Context, request Request, publish func(string, string) error) (result Result, err error) {
	return runWithBeforePublish(ctx, request, publish, nil)
}

func runWithBeforePublish(ctx context.Context, request Request, publish func(string, string) error, beforePublish func()) (result Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	matcher := request.Matcher
	force := request.Force
	dryRun := request.DryRun
	resolved, err := ResolveRequest(request.Source, request.Output)
	if err != nil {
		return Result{}, err
	}
	request = resolved
	request.Matcher = matcher
	request.Force = force
	request.DryRun = dryRun

	if !request.DryRun && !request.Force {
		if _, statErr := os.Lstat(request.Output); statErr == nil {
			return Result{}, outputExistsError(request.Output)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return Result{}, fmt.Errorf("inspect output %q: %w", request.Output, statErr)
		}
	}

	if request.DryRun {
		stats, traverseErr := traverse(ctx, request.Source, request.Output, "", request.Matcher, nil)
		return Result{Stats: stats}, traverseErr
	}

	destination := filepath.Dir(request.Output)
	tempFile, err := os.CreateTemp(destination, ".zipit-*.tmp")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary output in %q: %w", destination, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if tempFile != nil {
			_ = tempFile.Close()
		}
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	zipWriter := zip.NewWriter(tempFile)
	result.Stats, err = traverse(ctx, request.Source, request.Output, tempPath, request.Matcher, func(path, name string, info fs.FileInfo) error {
		return writeZipEntry(ctx, zipWriter, path, name, info)
	})
	if err != nil {
		_ = zipWriter.Close()
		return Result{}, err
	}
	if err = zipWriter.Close(); err != nil {
		return Result{}, fmt.Errorf("close ZIP writer: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return Result{}, fmt.Errorf("close temporary output: %w", err)
	}
	tempFile = nil
	if contextErr := ctx.Err(); contextErr != nil {
		return Result{}, fmt.Errorf("archive interrupted: %w", contextErr)
	}

	info, statErr := os.Stat(tempPath)
	if statErr != nil {
		return Result{}, fmt.Errorf("inspect completed archive: %w", statErr)
	}
	result.ArchiveSize = info.Size()

	if !request.Force {
		if _, statErr := os.Lstat(request.Output); statErr == nil {
			return Result{}, outputExistsError(request.Output)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return Result{}, fmt.Errorf("inspect output %q: %w", request.Output, statErr)
		}
	}
	if beforePublish != nil {
		beforePublish()
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return Result{}, fmt.Errorf("archive interrupted: %w", contextErr)
	}
	if err = publish(tempPath, request.Output); err != nil {
		if !request.Force {
			if _, statErr := os.Lstat(request.Output); statErr == nil {
				return Result{}, outputExistsError(request.Output)
			}
		}
		return Result{}, fmt.Errorf("publish output %q: %w", request.Output, err)
	}
	return result, nil
}

func outputExistsError(path string) error {
	return fmt.Errorf("output already exists: %s\nuse --force to replace it", path)
}

type entrySink func(path, name string, info fs.FileInfo) error

func traverse(ctx context.Context, source, output, tempPath string, matcher Matcher, sink entrySink) (stats Stats, err error) {
	var walkDirectory func(string, fs.FileInfo) error
	walkDirectory = func(directory string, expected fs.FileInfo) error {
		if contextErr := ctx.Err(); contextErr != nil {
			return fmt.Errorf("archive interrupted: %w", contextErr)
		}
		directoryHandle, openErr := openDirectoryNoFollow(directory)
		if openErr != nil {
			return fmt.Errorf("open directory %q: %w", directory, openErr)
		}
		defer directoryHandle.Close()
		openedInfo, statErr := directoryHandle.Stat()
		if statErr != nil {
			return fmt.Errorf("inspect opened directory %q: %w", directory, statErr)
		}
		if !openedInfo.IsDir() || (expected != nil && !os.SameFile(expected, openedInfo)) {
			return fmt.Errorf("directory changed during archive creation: %q", directory)
		}
		entries, readErr := directoryHandle.ReadDir(-1)
		if readErr != nil {
			return fmt.Errorf("read directory %q: %w", directory, readErr)
		}

		for _, entry := range entries {
			if contextErr := ctx.Err(); contextErr != nil {
				return fmt.Errorf("archive interrupted: %w", contextErr)
			}
			path := filepath.Join(directory, entry.Name())
			if samePath(path, output) || (tempPath != "" && samePath(path, tempPath)) {
				continue
			}
			if changedErr := ensureDirectoryUnchanged(directory, openedInfo); changedErr != nil {
				return changedErr
			}

			relative, relativeErr := filepath.Rel(source, path)
			if relativeErr != nil {
				return fmt.Errorf("make %q relative to source: %w", path, relativeErr)
			}
			name := filepath.ToSlash(relative)
			if name == "." || strings.HasPrefix(name, "../") || name == ".." || filepath.IsAbs(relative) {
				return fmt.Errorf("unsafe archive path %q", relative)
			}

			info, infoErr := entry.Info()
			if infoErr != nil {
				return fmt.Errorf("inspect %q: %w", path, infoErr)
			}
			if changedErr := ensureDirectoryUnchanged(directory, openedInfo); changedErr != nil {
				return changedErr
			}
			mode := info.Mode()
			if mode&os.ModeSymlink != 0 {
				if matcher != nil && matcher.Match(name, false) {
					continue
				}
				stats.SymlinksIncluded++
				stats.BytesIncluded += info.Size()
			} else if !mode.IsRegular() && !mode.IsDir() {
				stats.SpecialFilesSkipped++
				continue
			} else if mode.IsDir() {
				if matcher != nil && matcher.Match(name, true) {
					stats.DirectoriesExcluded++
					continue
				}
			} else {
				if matcher != nil && matcher.Match(name, false) {
					stats.FilesExcluded++
					continue
				}
				stats.FilesIncluded++
				stats.BytesIncluded += info.Size()
			}
			if sink != nil {
				if sinkErr := sink(path, name, info); sinkErr != nil {
					return sinkErr
				}
				if changedErr := ensureDirectoryUnchanged(directory, openedInfo); changedErr != nil {
					return changedErr
				}
			}
			if mode.IsDir() {
				if recurseErr := walkDirectory(path, info); recurseErr != nil {
					return recurseErr
				}
				if changedErr := ensureDirectoryUnchanged(directory, openedInfo); changedErr != nil {
					return changedErr
				}
			}
		}
		return nil
	}
	err = walkDirectory(source, nil)
	return stats, err
}

func ensureDirectoryUnchanged(path string, expected fs.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("reinspect directory %q: %w", path, err)
	}
	if !current.IsDir() || !os.SameFile(expected, current) {
		return fmt.Errorf("directory changed during archive creation: %q", path)
	}
	return nil
}

func writeZipEntry(ctx context.Context, writer *zip.Writer, path, name string, info fs.FileInfo) error {
	mode := info.Mode()
	if mode.IsDir() {
		name += "/"
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create ZIP header for %q: %w", path, err)
	}
	header.Name = name
	header.SetMode(mode)
	header.Modified = info.ModTime()
	if mode.IsRegular() {
		header.Method = zip.Deflate
	} else if mode&os.ModeSymlink != 0 {
		header.Method = zip.Store
	}
	entryWriter, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create ZIP entry %q: %w", name, err)
	}
	if mode.IsDir() {
		return nil
	}
	if mode&os.ModeSymlink != 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("archive interrupted: %w", err)
		}
		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("read symlink %q: %w", path, err)
		}
		if _, err := io.WriteString(entryWriter, target); err != nil {
			return fmt.Errorf("archive symlink %q: %w", path, err)
		}
		return nil
	}

	file, err := openRegularNoFollow(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	currentInfo, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return fmt.Errorf("inspect opened file %q: %w", path, statErr)
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(info, currentInfo) {
		_ = file.Close()
		return fmt.Errorf("file changed during archive creation: %q", path)
	}
	_, copyErr := copyWithContext(ctx, entryWriter, file)
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("archive %q: %w", path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %q: %w", path, closeErr)
	}
	return nil
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, fmt.Errorf("archive interrupted: %w", err)
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := destination.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
