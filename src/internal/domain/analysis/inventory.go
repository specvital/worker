package analysis

type Inventory struct {
	Files []TestFile
}

type TestFile struct {
	Path      string
	Framework string
	Suites    []TestSuite
	Tests     []Test
}

type TestSuite struct {
	Name     string
	Location Location
	Suites   []TestSuite
	Tests    []Test
}

type Test struct {
	Name     string
	Location Location
	Status   TestStatus
}

type Location struct {
	StartLine int
	EndLine   int
}

type TestStatus string

const (
	TestStatusActive  TestStatus = "active"
	TestStatusSkipped TestStatus = "skipped"
	TestStatusTodo    TestStatus = "todo"
)
