package cli

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/poizdev/zipit/internal/config"
	"github.com/poizdev/zipit/internal/presets"
)

func TestConfigAddNormalizesCreatesAndDeduplicates(t *testing.T) {
	path := isolateConfigPath(t)
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "add", `.\cache\`})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	command = NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "add", "./cache/"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "cache/" {
		t.Fatalf("patterns = %v, want one canonical rule", got.Ignore.Patterns)
	}
	if !strings.Contains(output.String(), "Added: cache/") || !strings.Contains(output.String(), "Already configured: cache/") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestConfigAddRejectsInvalidRuleWithoutWriting(t *testing.T) {
	path := isolateConfigPath(t)
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "add", "["})
	if err := command.Execute(); err == nil {
		t.Fatal("config add error = nil")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config exists or stat failed: %v", err)
	}
}

func TestConfigAddPreservesExistingPatterns(t *testing.T) {
	path := isolateConfigPath(t)
	value := config.Empty()
	value.Ignore.Patterns = []string{"keep/"}
	if err := config.Write(path, value); err != nil {
		t.Fatal(err)
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "add", "*.log"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "keep/,*.log" {
		t.Fatalf("patterns = %v", got.Ignore.Patterns)
	}
}

func TestConfigRemoveCanonicalRuleAndPreservesOthers(t *testing.T) {
	path := isolateConfigPath(t)
	value := config.Empty()
	value.Ignore.Patterns = []string{"cache/", "*.log"}
	if err := config.Write(path, value); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "remove", `.\cache\`})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "*.log" || !strings.Contains(output.String(), "Removed: cache/") {
		t.Fatalf("config = %+v, output = %q", got, output.String())
	}
}

func TestConfigRemoveMissingDoesNotCreateOrRewrite(t *testing.T) {
	path := isolateConfigPath(t)
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "remove", "*.log"})
	if err := command.Execute(); err == nil {
		t.Fatal("remove missing config error = nil")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config exists or stat failed: %v", err)
	}
}

func TestConfigRemoveAbsentRuleDoesNotRewrite(t *testing.T) {
	path := isolateConfigPath(t)
	value := config.Empty()
	value.Ignore.Patterns = []string{"keep/"}
	if err := config.Write(path, value); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "remove", "*.log"})
	if err := command.Execute(); err == nil {
		t.Fatal("remove absent rule error = nil")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("config changed after removing an absent rule")
	}
}

func TestConfigListShowsOnlyPersistedGlobalRules(t *testing.T) {
	path := isolateConfigPath(t)
	value := config.Empty()
	value.Ignore.Patterns = []string{".git/", "*.log"}
	if err := config.Write(path, value); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "list"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Global excludes", "1. .git/", "2. *.log", path} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output %q missing %q", output.String(), want)
		}
	}
}

func TestConfigListMissingDoesNotCreateFile(t *testing.T) {
	path := isolateConfigPath(t)
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "list"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No global excludes configured") || !strings.Contains(output.String(), path) {
		t.Fatalf("output = %q", output.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("list created config or stat failed: %v", err)
	}
}

func TestConfigListExistingEmptyConfig(t *testing.T) {
	path := isolateConfigPath(t)
	if err := config.Write(path, config.Empty()); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config", "list"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "No global excludes configured") || !strings.Contains(output.String(), path) {
		t.Fatalf("output = %q", output.String())
	}
}

func TestBuildInitialPatternsExpandsValidatesAndDeduplicates(t *testing.T) {
	got, err := buildInitialPatterns([]presets.Preset{presetByID(t, "git"), presetByID(t, "os")}, []string{"*.log", ".git/"})
	if err != nil {
		t.Fatal(err)
	}
	want := ".git/,.DS_Store,Thumbs.db,*.log"
	if strings.Join(got, ",") != want {
		t.Fatalf("patterns = %v, want %s", got, want)
	}
	if _, err := buildInitialPatterns(nil, []string{"["}); err == nil {
		t.Fatal("invalid custom rule accepted")
	}
	empty, err := buildInitialPatterns(nil, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty init = %v, %v", empty, err)
	}
}

func TestChooseExclusionRulesDefaultsAndEmptySelections(t *testing.T) {
	for _, test := range []struct {
		name     string
		selected []presets.Preset
		want     string
	}{
		{name: "defaults", selected: defaultPresets(), want: ".git/,.DS_Store,Thumbs.db"},
		{name: "empty", selected: nil, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			interaction := &fakeExclusionInteraction{selected: test.selected}
			got, err := chooseExclusionRules(interaction)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != test.want {
				t.Fatalf("patterns = %v, want %q", got, test.want)
			}
		})
	}
}

func TestChooseExclusionRulesAppendsValidatedCustomRulesAndDeduplicates(t *testing.T) {
	interaction := &fakeExclusionInteraction{
		selected: []presets.Preset{presetByID(t, "git")},
		confirms: []bool{true, true, false},
		inputs:   []string{"[", "recordings/", ".git/", "*.mp4"},
	}
	got, err := chooseExclusionRules(interaction)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != ".git/,recordings/" {
		t.Fatalf("patterns = %v", got)
	}
	if interaction.validationErrors != 1 {
		t.Fatalf("validation errors = %d, want 1", interaction.validationErrors)
	}
}

func TestRunConfigInitCancellationCreatesNoConfig(t *testing.T) {
	path := isolateConfigPath(t)
	interaction := &fakeExclusionInteraction{selected: nil, confirms: []bool{false}}
	var output bytes.Buffer
	if err := runConfigInit(&output, path, interaction); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cancelled init created config: %v", err)
	}
}

func TestRunConfigInitWritesReloadableSchemaOneConfig(t *testing.T) {
	path := isolateConfigPath(t)
	interaction := &fakeExclusionInteraction{
		selected: []presets.Preset{presetByID(t, "os")},
		confirms: []bool{false, true},
	}
	var output bytes.Buffer
	if err := runConfigInit(&output, path, interaction); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != 1 || strings.Join(got.Ignore.Patterns, ",") != ".DS_Store,Thumbs.db" {
		t.Fatalf("config = %+v", got)
	}
}

func TestRunConfigInitDoesNotReplaceConfigCreatedDuringInteraction(t *testing.T) {
	path := isolateConfigPath(t)
	interaction := &fakeExclusionInteraction{
		selected: []presets.Preset{presetByID(t, "git")},
		confirms: []bool{false, true},
		beforeConfirm: func(title string) {
			if title != "Create config?" {
				return
			}
			value := config.Empty()
			value.Ignore.Patterns = []string{"keep/"}
			if err := config.Write(path, value); err != nil {
				t.Fatal(err)
			}
		},
	}
	err := runConfigInit(&bytes.Buffer{}, path, interaction)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("runConfigInit() error = %v", err)
	}
	got, loadErr := config.Load(path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "keep/" {
		t.Fatalf("patterns = %v, want concurrent config preserved", got.Ignore.Patterns)
	}
}

type fakeExclusionInteraction struct {
	selected         []presets.Preset
	selectErr        error
	confirms         []bool
	inputs           []string
	validationErrors int
	beforeConfirm    func(string)
}

func (f *fakeExclusionInteraction) selectPresets([]presets.Preset) ([]presets.Preset, error) {
	return f.selected, f.selectErr
}

func (f *fakeExclusionInteraction) confirm(title string) (bool, error) {
	if f.beforeConfirm != nil {
		f.beforeConfirm(title)
	}
	if len(f.confirms) == 0 {
		return false, nil
	}
	answer := f.confirms[0]
	f.confirms = f.confirms[1:]
	return answer, nil
}

func (f *fakeExclusionInteraction) input(_ string, validate func(string) error) (string, error) {
	for len(f.inputs) > 0 {
		value := f.inputs[0]
		f.inputs = f.inputs[1:]
		if err := validate(value); err != nil {
			f.validationErrors++
			continue
		}
		return value, nil
	}
	return "", nil
}

func presetByID(t *testing.T, id string) presets.Preset {
	t.Helper()
	for _, preset := range presets.All() {
		if preset.ID == id {
			return preset
		}
	}
	t.Fatalf("missing preset %q", id)
	return presets.Preset{}
}

func TestConfigInitRefusesExistingAndNonInteractive(t *testing.T) {
	path := isolateConfigPath(t)
	value := config.Empty()
	value.Ignore.Patterns = []string{"keep/"}
	if err := config.Write(path, value); err != nil {
		t.Fatal(err)
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "init"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing init error = %v", err)
	}
	got, _ := config.Load(path)
	if strings.Join(got.Ignore.Patterns, ",") != "keep/" {
		t.Fatal("existing config was modified")
	}

	path = isolateConfigPath(t)
	command = NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetIn(strings.NewReader(""))
	command.SetArgs([]string{"config", "init"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("non-interactive init error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("non-interactive init created config: %v", err)
	}
}

func TestResolveEditorPrefersVisualAndSplitsArguments(t *testing.T) {
	t.Setenv("VISUAL", `"/Applications/Visual Studio Code.app/code" --wait --label "Zipit config"`)
	t.Setenv("EDITOR", "nvim")
	name, args, err := resolveEditor()
	if err != nil {
		t.Fatal(err)
	}
	if name != "/Applications/Visual Studio Code.app/code" || strings.Join(args, ",") != "--wait,--label,Zipit config" {
		t.Fatalf("editor = %q %v", name, args)
	}
}

func TestResolveEditorRejectsMalformedQuoting(t *testing.T) {
	t.Setenv("VISUAL", `"unterminated`)
	t.Setenv("EDITOR", "nvim")
	if _, _, err := resolveEditor(); err == nil {
		t.Fatal("malformed VISUAL error = nil")
	}
}

func TestCharacterDeviceIsNotAssumedToBeTerminal(t *testing.T) {
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("open null device: %v", err)
	}
	defer device.Close()
	if isInteractive(device) {
		t.Fatal("null device reported as interactive terminal")
	}
}

func TestConfigEditRequiresEditor(t *testing.T) {
	path := isolateConfigPath(t)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "edit"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "No editor configured") || !strings.Contains(err.Error(), path) {
		t.Fatalf("edit error = %v", err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
}

func isolateConfigPath(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("AppData", root)
	case "darwin":
		t.Setenv("HOME", root)
	default:
		t.Setenv("XDG_CONFIG_HOME", root)
	}
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigCommandShowsHelp(t *testing.T) {
	isolateConfigPath(t)
	var output bytes.Buffer
	command := NewRootCommand(&output, &bytes.Buffer{})
	command.SetArgs([]string{"config"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "add") || !strings.Contains(output.String(), "edit") {
		t.Fatalf("config help = %q", output.String())
	}
}

func TestEditorHelperProcess(t *testing.T) {
	if os.Getenv("ZIPIT_EDITOR_HELPER") != "1" {
		return
	}
	path := os.Args[len(os.Args)-1]
	value := config.Empty()
	if os.Getenv("ZIPIT_EDITOR_INVALID") == "1" {
		value.Ignore.Patterns = []string{"["}
	} else {
		value.Ignore.Patterns = []string{"*.edited"}
	}
	if err := config.Write(path, value); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestConfigEditLaunchesWaitsAndValidates(t *testing.T) {
	path := isolateConfigPath(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZIPIT_EDITOR_HELPER", "1")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", executable+" -test.run=TestEditorHelperProcess --")
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "edit"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Ignore.Patterns, ",") != "*.edited" {
		t.Fatalf("edited patterns = %v", got.Ignore.Patterns)
	}
}

func TestConfigEditReportsInvalidEditedRules(t *testing.T) {
	path := isolateConfigPath(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZIPIT_EDITOR_HELPER", "1")
	t.Setenv("ZIPIT_EDITOR_INVALID", "1")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", executable+" -test.run=TestEditorHelperProcess --")
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	command.SetArgs([]string{"config", "edit"})
	err = command.Execute()
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("edit invalid config error = %v", err)
	}
	got, loadErr := config.Load(path)
	if loadErr != nil || strings.Join(got.Ignore.Patterns, ",") != "[" {
		t.Fatalf("edited file was unexpectedly reverted: config=%+v error=%v", got, loadErr)
	}
}
