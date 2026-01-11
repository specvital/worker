package mapping

import (
	"github.com/specvital/core/pkg/domain"
	"github.com/specvital/worker/internal/domain/analysis"
)

// ConvertCoreToDomainInventory converts inventory from specvital/core types to domain types.
func ConvertCoreToDomainInventory(coreInv *domain.Inventory) *analysis.Inventory {
	if coreInv == nil {
		return nil
	}

	domainFiles := make([]analysis.TestFile, 0, len(coreInv.Files))
	for _, coreFile := range coreInv.Files {
		domainFiles = append(domainFiles, convertCoreTestFile(coreFile))
	}

	return &analysis.Inventory{
		Files: domainFiles,
	}
}

func convertCoreTestFile(coreFile domain.TestFile) analysis.TestFile {
	domainSuites := make([]analysis.TestSuite, 0, len(coreFile.Suites))
	for _, coreSuite := range coreFile.Suites {
		domainSuites = append(domainSuites, convertCoreTestSuite(coreSuite))
	}

	domainTests := make([]analysis.Test, 0, len(coreFile.Tests))
	for _, coreTest := range coreFile.Tests {
		domainTests = append(domainTests, convertCoreTest(coreTest))
	}

	return analysis.TestFile{
		DomainHints: convertDomainHints(coreFile.DomainHints),
		Framework:   coreFile.Framework,
		Path:        coreFile.Path,
		Suites:      domainSuites,
		Tests:       domainTests,
	}
}

func convertDomainHints(coreHints *domain.DomainHints) *analysis.DomainHints {
	if coreHints == nil {
		return nil
	}
	return &analysis.DomainHints{
		Calls:   coreHints.Calls,
		Imports: coreHints.Imports,
	}
}

func convertCoreTestSuite(coreSuite domain.TestSuite) analysis.TestSuite {
	domainSuites := make([]analysis.TestSuite, 0, len(coreSuite.Suites))
	for _, nested := range coreSuite.Suites {
		domainSuites = append(domainSuites, convertCoreTestSuite(nested))
	}

	domainTests := make([]analysis.Test, 0, len(coreSuite.Tests))
	for _, coreTest := range coreSuite.Tests {
		domainTests = append(domainTests, convertCoreTest(coreTest))
	}

	return analysis.TestSuite{
		Name: coreSuite.Name,
		Location: analysis.Location{
			StartLine: coreSuite.Location.StartLine,
			EndLine:   coreSuite.Location.EndLine,
		},
		Suites: domainSuites,
		Tests:  domainTests,
	}
}

func convertCoreTest(coreTest domain.Test) analysis.Test {
	return analysis.Test{
		Name: coreTest.Name,
		Location: analysis.Location{
			StartLine: coreTest.Location.StartLine,
			EndLine:   coreTest.Location.EndLine,
		},
		Status: convertCoreTestStatus(coreTest.Status),
	}
}

func convertCoreTestStatus(coreStatus domain.TestStatus) analysis.TestStatus {
	switch coreStatus {
	case domain.TestStatusFocused:
		return analysis.TestStatusFocused
	case domain.TestStatusSkipped:
		return analysis.TestStatusSkipped
	case domain.TestStatusTodo:
		return analysis.TestStatusTodo
	case domain.TestStatusXfail:
		return analysis.TestStatusXfail
	default:
		return analysis.TestStatusActive
	}
}
