package build

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CompileConfig wraps common compiler configuration options
type CompileConfig struct {
	// The Development options is passed to build steps
	Development bool

	// The Quick option is passed to build steps
	Quick bool

	// Version contains the version number for the software package, if one can be determined
	Version string

	GOOS   string
	GOARCH string
}

type job func(ctx context.Context) error

// CompilerJob wraps a build step.
type CompilerJob func(ctx context.Context, conf CompileConfig) error

// WatchList wraps a source tree to be watched for changes
type WatchList struct {
	// Paths contains the list of all files and directories to watch
	Paths []string

	// FileFilter limits the watched paths to only files matching one of these filters (e.g. "*.go")
	FileFilter []string
}

// Main runs the build script. Calling Main will also call flag.Parse().
func Main(executableName string, compile CompilerJob, watchList WatchList) {
	var conf CompileConfig
	watch := false
	run := false
	flag.BoolVar(&conf.Development, "development", false, "Create a development build")
	flag.BoolVar(&conf.Quick, "quick", false, "Create a development build")
	flag.StringVar(&conf.GOARCH, "GOARCH", "", "Cross-compile for architecture")
	flag.StringVar(&conf.GOOS, "GOOS", "", "Cross-compile for operating system")
	flag.BoolVar(&watch, "watch", false, "Watch source tree for changes")
	flag.BoolVar(&run, "run", false, "Run "+executableName+" upon successful compilation")
	flag.Parse()

	if conf.Development && conf.Quick {
		//log.Printf("")
		//log.Printf("You requested a quick build. This will assume")
		//log.Printf(" you have a version of  `sassc`  running")
		//log.Printf(" in a separate process.")
		//log.Printf("")
	}

	compile = withVersion(compile)

	var theJob job

	if run {
		theJob = func(ctx context.Context) error {
			err := compile(ctx, conf)
			if err != nil {
				return err
			}
			runArgs := append([]string{executableName}, flag.Args()...)
			return Passthru(ctx, runArgs...)
		}
	} else {
		theJob = func(ctx context.Context) error {
			return compile(ctx, conf)
		}
	}

	if watch {
		theJob = watchSourceTree(watchList, theJob)
	}

	err := theJob(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func withVersion(compile CompilerJob) CompilerJob {
	return func(ctx context.Context, conf CompileConfig) error {
		// Determine version
		conf.Version = "unknown-version"
		gitDescCmd := exec.CommandContext(ctx, "git", "describe")
		gitDescribe, err := gitDescCmd.Output()
		if err == nil && len(gitDescribe) > 0 {
			conf.Version = strings.TrimLeft(strings.TrimSpace(string(gitDescribe)), "v")
		}
		return compile(ctx, conf)
	}
}

// Passthru executes the command and arguments in argv, and returns an error if
// the exit status wasn't 0. Stdin, stdout, and stderr are redirected to the
// parent process' stdin/stdout/stderr.
func Passthru(ctx context.Context, argv ...string) error {
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	return PassthruCmd(c)
}

// PassthruCmd runs the supplied Cmd, and returns an error if the exit status
// wasn't 0. Stdin, stdout, and stderr are redirected to the parent process'
// stdin/stdout/stderr.
func PassthruCmd(c *exec.Cmd) error {
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func watchSourceTree(watchList WatchList, childJob job) job {
	return func(ctx context.Context) error {
		var mu sync.Mutex
		for {
			lastHash := sourceTreeHash(watchList)
			current := lastHash
			cctx, cancel := context.WithCancel(ctx)
			go func() {
				mu.Lock()
				err := childJob(cctx)
				if err != nil {
					log.Printf("child process: %s", err)
				}
				mu.Unlock()
			}()

			for lastHash == current {
				time.Sleep(250 * time.Millisecond)
				current = sourceTreeHash(watchList)
			}

			log.Printf("Source change detected - rebuilding")
			cancel()
		}
	}
}

func sourceTreeHash(w WatchList) string {
	h := sha1.New()
	for _, d := range w.Paths {
		h.Write(directoryHash(0, d, w))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func directoryHash(level int, filePath string, w WatchList) []byte {
	h := sha1.New()
	h.Write([]byte(filePath))

	fi, err := os.Stat(filePath)
	if err != nil {
		return h.Sum(nil)
	}
	if fi.IsDir() {
		base := filepath.Base(filePath)
		if level > 0 {
			if base == ".git" || base == ".." || base == "node_modules" || base == "build" || base == "doc" {
				return []byte{}
			}
		}
		// recurse
		var names []string
		f, err := os.Open(filePath)
		if err == nil {
			names, err = f.Readdirnames(-1)
		}
		if err == nil {
			for _, name := range names {
				if name == "" || name[0] == '.' {
					continue
				}
				h.Write(directoryHash(level+1, path.Join(filePath, name), w))
			}
		}
	} else {
		if w.FileFilter != nil {
			found := false
			for _, pattern := range w.FileFilter {
				if ok, _ := filepath.Match(pattern, filePath); ok {
					found = true
				} else if ok, _ := filepath.Match(pattern, filepath.Base(filePath)); ok {
					found = true
				}
			}
			if !found {
				return []byte{}
			}
		}
		f, err := os.Open(filePath)
		if err == nil {
			io.Copy(h, f)
			f.Close()
		}
	}
	return h.Sum(nil)
}
