package gochk

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	pfmt     = "\"fmt\""
	pstrings = "\"strings\""

	testDirPath         = "../../test/data/test"
	underscoreTestPath  = "../../test/data/_test.go"
	blockCommentsPath   = "../../test/data/blockComments.go"
	lockfilePath        = "../../test/data/lockfile.txt"
	lockedPath          = "../../test/data/adapter/locked.go"
	violatefilePath     = "../../test/data/violatefile.txt"
	violatePath         = "../../test/data/application/violate.go"
	multipleImportsPath = "../../test/data/multipleImports.go"
	oneImportPath       = "../../test/data/oneImport.go"
	singleCommentsPath  = "../../test/data/singleComments.go"
	firstLayerPath      = "../../test/data/domain/firstLayer.go"
	secondLayerPath     = "../../test/data/application/secondLayer.go"
	fourthLayerPath     = "../../test/data/external/fourthLayer.go"

	adapterPkgPath     = "\"github.com/resotto/gochk/test/data/adapter\""
	applicationPkgPath = "\"github.com/resotto/gochk/test/data/application\""
	domainPkgPath      = "\"github.com/resotto/gochk/test/data/domain\""
)

var (
	ignorePaths      = []string{"test", "_test"}
	dependencyOrders = []string{"external", "adapter", "application", "domain"}
)

func createFile(path string, contentsPath string, permission os.FileMode) string {
	contentsFilepath, _ := filepath.Abs(contentsPath)
	bytes, rerr := ioutil.ReadFile(contentsFilepath)
	if rerr != nil {
		panic(rerr)
	}
	filepath, _ := filepath.Abs(path)
	ioutil.WriteFile(filepath, bytes, permission)
	return filepath
}

func removeFile(path string) {
	filepath, _ := filepath.Abs(path)
	err := os.Remove(filepath)
	if err != nil {
		panic(err)
	}
}

func mkdir(path string, permission os.FileMode) {
	filepath, _ := filepath.Abs(path)
	err := os.Mkdir(filepath, permission)
	if err != nil {
		panic(err)
	}
}

func rmdir(path string) {
	filepath, _ := filepath.Abs(path)
	err := os.Remove(filepath)
	if err != nil {
		panic(err)
	}
}

func setup() {
	mkdir(testDirPath, os.ModePerm)
	createFile(lockedPath, lockfilePath, 0300)
	createFile(violatePath, violatefilePath, os.ModePerm)
}

func teardown() {
	rmdir(testDirPath)
	removeFile(lockedPath)
	removeFile(violatePath)
}

func TestMain(m *testing.M) {
	setup()
	result := m.Run()
	teardown()
	os.Exit(result)
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected []CheckResult
	}{
		{
			"violation found",
			Config{
				TargetPath:       violatePath,
				DependencyOrders: dependencyOrders,
				Ignore:           []string{},
			},
			[]CheckResult{
				CheckResult{resultType: violated, message: "not tested", color: red},
			},
		},
		{
			"no results",
			Config{
				TargetPath:       violatefilePath,
				DependencyOrders: dependencyOrders,
				Ignore:           []string{},
			},
			[]CheckResult{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := Check(tt.cfg)
			if len(results) != len(tt.expected) {
				t.Errorf("got %d, want %d", len(results), len(tt.expected))
			}
			for i, r := range results {
				if r.resultType != tt.expected[i].resultType {
					t.Errorf("got %s, want %s", r.resultType, tt.expected[i].resultType)
				}
				if r.color != tt.expected[i].color {
					t.Errorf("got %s, want %s", r.color, tt.expected[i].color)
				}
			}
		})
	}
}

func TestMatchIgnore(t *testing.T) {
	type result struct {
		matched bool
		err     error
	}
	tests := []struct {
		name        string
		ignorePaths []string
		targetPath  string
		expected    result
	}{
		{
			"test dir",
			ignorePaths,
			testDirPath,
			result{matched: true, err: filepath.SkipDir},
		},
		{
			"_test.go file",
			ignorePaths,
			underscoreTestPath,
			result{matched: true, err: nil},
		},
		{
			".go file which is not included in ignore",
			ignorePaths,
			"./print.go",
			result{matched: false, err: nil},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := filepath.Walk(tt.targetPath, func(path string, info os.FileInfo, err error) error {
				matched, err := matchIgnore(tt.ignorePaths, path, info)
				if matched != tt.expected.matched {
					t.Errorf("got %t, want %t", matched, tt.expected.matched)
				}
				if err != tt.expected.err {
					t.Errorf("got %s, want %s", err, tt.expected.err)
				}
				return nil
			})
			if err != nil {
				panic(err)
			}
		})
	}
}

func TestRetrieveDependencies(t *testing.T) {
	tests := []struct {
		name         string
		dependencies []string
		path         string
		currentLayer int
		expected     []dependency
	}{
		{
			"one layer retrieval",
			dependencyOrders,
			fourthLayerPath,
			0,
			[]dependency{
				dependency{fourthLayerPath, 0, domainPkgPath, 3},
			},
		},
		{
			"include file which couldn't be opened",
			dependencyOrders,
			lockedPath,
			1,
			[]dependency{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results, _ := retrieveDependencies(tt.dependencies, tt.path, tt.currentLayer)
			for i, r := range results {
				if r.filePath != tt.expected[i].filePath {
					t.Errorf("got %s, want %s", r.filePath, tt.expected[i].filePath)
				}
				if r.fileLayer != tt.expected[i].fileLayer {
					t.Errorf("got %d, want %d", r.fileLayer, tt.expected[i].fileLayer)
				}
				if r.importPath != tt.expected[i].importPath {
					t.Errorf("got %s, want %s", r.importPath, tt.expected[i].importPath)
				}
				if r.importLayer != tt.expected[i].importLayer {
					t.Errorf("got %d, want %d", r.importLayer, tt.expected[i].importLayer)
				}
			}
		})
	}
}

func TestReadImports(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		expected []string
	}{
		{
			"single import",
			oneImportPath,
			[]string{pfmt},
		},
		{
			"multiple import",
			multipleImportsPath,
			[]string{pfmt, pstrings},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filepath, _ := filepath.Abs(tt.filepath)
			f, err := os.Open(filepath)
			defer f.Close()
			if err != nil {
				t.Errorf("couldn't open file: %s", filepath)
			}
			results := readImports(f)
			for i, importPath := range results {
				if importPath != tt.expected[i] {
					t.Errorf("got %s, want %s", importPath, tt.expected[i])
				}
			}
		})
	}
}

func TestSkipToImportStatement(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		expected bool
	}{
		{
			"with block comments",
			blockCommentsPath,
			true,
		},
		{
			"without block comments",
			singleCommentsPath,
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filepath, _ := filepath.Abs(tt.filepath)
			f, err := os.Open(filepath)
			defer f.Close()
			if err != nil {
				t.Errorf("couldn't open file: %s", filepath)
				return
			}
			scanner := bufio.NewScanner(f)
			skipToImportStatement(scanner)
			scanner.Scan()
			if line := scanner.Text(); len(line) <= 6 || !strings.EqualFold(line[:6], "import") {
				t.Errorf("didn't skip to import statement: %s", line)
			}
		})
	}
}

func TestSkipBlockComments(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		expected bool
	}{
		{
			"block comments included and scanner.Scan() should return true",
			blockCommentsPath,
			true,
		},
		{
			"no block comments and scanner.Scan() should return true",
			oneImportPath,
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filepath, _ := filepath.Abs(tt.filepath)
			f, err := os.Open(filepath)
			defer f.Close()
			if err != nil {
				t.Errorf("couldn't open file: %s", filepath)
				return
			}
			scanner := bufio.NewScanner(f)
			scanner.Scan()
			line := scanner.Text()
			if skipBlockComments(line, scanner); scanner.Scan() != tt.expected {
				t.Errorf("got %t, want %t", !tt.expected, tt.expected)
			}
		})
	}
}

func TestRetrieveMultipleImportPath(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		expected []string
	}{
		{
			"multiple import path",
			multipleImportsPath,
			[]string{pfmt, pstrings},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filepath, _ := filepath.Abs(tt.filepath)
			f, err := os.Open(filepath)
			defer f.Close()
			if err != nil {
				t.Errorf("couldn't open file: %s", filepath)
				return
			}
			scanner := bufio.NewScanner(f)
			var line string
			for scanner.Scan() {
				if line = scanner.Text(); len(line) > 8 && strings.EqualFold(line[:8], "import (") {
					break
				}
			}
			results := retrieveMultipleImportPath(scanner, line)
			for i, r := range results {
				if r != tt.expected[i] {
					t.Errorf("got %s, want %s", r, tt.expected[i])
				}
			}
		})
	}
}