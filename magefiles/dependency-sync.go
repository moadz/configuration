package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/rhobs/configuration/internal/submodule"
)

type (
	Sync mg.Namespace
)

// Konflux syncs the given operator manifests from the specified ref.
// Ref can be a specific commit or "latest" to use the latest commit from upstream.
func (s Sync) Konflux(dependency string, ref string) error {
	switch dependency {
	case "thanos-operator":
		if err := s.syncThanosOperator(ref); err != nil {
			return fmt.Errorf("failed to sync Thanos Operator: %w", err)
		}
	case "loki-operator":
		if err := s.syncLokiOperator(ref); err != nil {
			return fmt.Errorf("failed to sync Loki Operator: %w", err)
		}
	case "observatorium-api":
		if err := s.syncObservatoriumAPI(ref); err != nil {
			return fmt.Errorf("failed to sync Observatorium API: %w", err)
		}
	case "tempo-operator":
		if err := s.syncTempoOperator(ref); err != nil {
			return fmt.Errorf("failed to sync Tempo Operator: %w", err)
		}
	default:
		return fmt.Errorf("unsupported dependency: %s", dependency)
	}

	return nil
}

// syncThanosOperator syncs Thanos Operator manifests from the given ref.
func (s Sync) syncThanosOperator(ref string) error {
	return dependencySyncer{
		konfluxRef: gitCommitRef{
			org:  "rhobs",
			repo: "rhobs-konflux-thanos-operator",
			ref:  ref,
		},
		submodule: "thanos-operator",
		imageTagVariable: goValue{
			filename: "clusters/template.go",
			name:     "ThanosOperatorVersion",
		},
		crdVersionVariable: &goValue{
			filename: "magefiles/thanos-operator.go",
			name:     "thanosOperatorCRDRef",
		},
	}.sync()
}

// syncLokiOperator syncs Thanos Operator manifests from the given ref.
func (s Sync) syncLokiOperator(ref string) error {
	return dependencySyncer{
		konfluxRef: gitCommitRef{
			org:  "rhobs",
			repo: "rhobs-konflux-loki-operator",
			ref:  ref,
		},
		submodule: "loki-operator",
		imageTagVariable: goValue{
			filename: "magefiles/loki-operator.go",
			name:     "lokiOperatorVersion",
		},
		crdVersionVariable: &goValue{
			filename: "magefiles/loki-operator.go",
			name:     "lokiOperatorCRDRef",
		},
		skipGoModUpdate: true,
	}.sync()
}

// syncObservatoriumAPI syncs Observatorium API from the given ref.
func (s Sync) syncObservatoriumAPI(ref string) error {
	return dependencySyncer{
		konfluxRef: gitCommitRef{
			org:  "rhobs",
			repo: "rhobs-konflux-obs-api",
			ref:  ref,
		},
		submodule: "api",
		imageTagVariable: goValue{
			filename: "clusters/template.go",
			name:     "ObservatoriumVersion",
		},
		skipGoModUpdate: true,
	}.sync()
}

// syncTempoOperator syncs Tempo Operator manifests from the given ref.
func (s Sync) syncTempoOperator(ref string) error {
	return dependencySyncer{
		konfluxRef: gitCommitRef{
			org:  "os-observability",
			repo: "konflux-tempo",
			ref:  ref,
		},
		submodule: "tempo-operator",
		imageTagVariable: goValue{
			filename: "magefiles/tempo-operator.go",
			name:     "tempoOperatorVersion",
		},
		crdVersionVariable: &goValue{
			filename: "magefiles/tempo-operator.go",
			name:     "tempoOperatorCRDRef",
		},
		skipGoModUpdate: true,
	}.sync()
}

// dependencySyncer synchronizes the image version and/or CRD manifests based on the dependency's
// submodule commit SHA in a Konflux repository.
//
// Currently supported repositories are
// - https://github.com/rhobs/rhobs-konflux-loki-operator
// - https://github.com/os-observability/konflux-tempo
// - https://github.com/rhobs/rhobs-konflux-thanos-operator
// - https://github.com/rhobs/rhobs-konflux-obs-api
type dependencySyncer struct {
	konfluxRef gitCommitRef
	submodule  string

	imageTagVariable   goValue
	crdVersionVariable *goValue

	skipGoModUpdate bool
}

type gitCommitRef struct {
	org  string
	repo string
	ref  string
}

// goValue represents a Go variable or const in a file.
type goValue struct {
	filename string
	name     string
}

func (v goValue) String() string {
	return fmt.Sprintf("%s (%s)", v.name, v.filename)
}

func (s dependencySyncer) sync() error {
	var (
		err       error
		component = path.Join(s.konfluxRef.org, s.konfluxRef.repo)
		apiURL    = &url.URL{
			Scheme: "https",
			Host:   "api.github.com",
			Path:   path.Join("/repos", component),
		}
		repoURL = &url.URL{
			Scheme: "https",
			Host:   "github.com",
			Path:   path.Join(component),
		}
	)

	ref := s.konfluxRef.ref
	if ref == "latest" {
		ref, err = submodule.GithubLatestCommit(apiURL.String())
		if err != nil {
			return fmt.Errorf("%s: failed to get latest commit SHA: %w", repoURL, err)
		}
		fmt.Fprintf(os.Stdout, "No ref provided, using latest commit from main: %s\n", ref)
	}

	fmt.Fprintf(os.Stdout, "Syncing %s at commit %s\n", component, ref)
	if err = s.imageTagVariable.updateConst(ref); err != nil {
		return fmt.Errorf("failed to update %q variable: %w", s.imageTagVariable, err)
	}

	info := submodule.Info{
		Commit:        ref,
		SubmodulePath: s.submodule,
		URL:           repoURL.String(),
	}
	module, err := info.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse submodule info: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Parsed submodule %s commit: %s\n", module.Path, module.Commit)
	if s.crdVersionVariable != nil {
		err = s.crdVersionVariable.updateConst(module.Commit)
		if err != nil {
			return fmt.Errorf("failed to update Thanos Operator CRD ref: %w", err)
		}
	}

	// Update go.mod with the commit SHA.
	if err = s.updateGoMod(module); err != nil {
		return fmt.Errorf("failed to update go.mod: %w", err)
	}

	return nil
}

func (s dependencySyncer) updateGoMod(gitSubmodule submodule.Module) error {
	if s.skipGoModUpdate {
		return nil
	}

	requireStanza := fmt.Sprintf("%s@%s", strings.TrimPrefix(gitSubmodule.URL, "https://"), gitSubmodule.Commit)
	cmd := exec.Command("go", "mod", "edit", "-require", requireStanza)
	fmt.Fprintf(os.Stdout, "Running: %s\n", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update go.mod: %w\nOutput: %s", err, string(output))
	}

	// Run go mod tidy to clean up
	cmd = exec.Command("go", "mod", "tidy")
	fmt.Fprintf(os.Stdout, "Running: %s\n", cmd.String())

	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run go mod tidy: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(os.Stdout, "Updated go.mod: %s\n", requireStanza)
	return nil
}

func (v goValue) updateConst(newRef string) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, v.filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	var found bool
	// Find and update the thanosOperatorRef constant
	ast.Inspect(node, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			for _, spec := range genDecl.Specs {
				if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range valueSpec.Names {
						if name.Name == v.name && i < len(valueSpec.Values) {
							if basicLit, ok := valueSpec.Values[i].(*ast.BasicLit); ok {
								basicLit.Value = strconv.Quote(newRef)
								found = true
								return false // Stop searching
							}
						}
					}
				}
			}
		}
		return true
	})

	if !found {
		return fmt.Errorf("failed to find variable %s", v)
	}

	// Write the updated AST back to file
	file, err := os.Create(v.filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := format.Node(file, fset, node); err != nil {
		return fmt.Errorf("failed to write formatted code: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Updated %s to: %s\n", v, newRef)
	return nil
}
