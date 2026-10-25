package screens

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"wslproxy/internal/config"
	"wslproxy/internal/tui/layout"
)

const formInputCharLimit = 256

const (
	listenAddressFieldIndex = iota
	listenPortFieldIndex
	distributionFieldIndex
	targetPortFieldIndex
	manualTargetIPFieldIndex
	connectTimeoutFieldIndex
	connectRetriesFieldIndex
	retryDelayFieldIndex
)

var fieldLabels = []string{"Windows listen address", "Windows listen port", "WSL distribution (Enter to select)", "WSL target port", "Manual IPv4 (empty = auto)", "Connect timeout (ms)", "TCP retries", "Retry delay (ms)"}

type Form struct {
	area     layout.Size
	receiver FormReceiver
	base     config.Config
	fields   []textinput.Model
	focus    int
}

func NewForm(receiver FormReceiver) *Form {
	return &Form{receiver: receiver}
}

func (form *Form) Open(cfg config.Config) tea.Cmd {
	form.base = cfg
	values := []string{cfg.ListenAddress, strconv.Itoa(cfg.ListenPort), cfg.Distribution, strconv.Itoa(cfg.TargetPort), cfg.ManualTargetIP, strconv.Itoa(cfg.ConnectTimeoutMS), strconv.Itoa(cfg.ConnectRetries), strconv.Itoa(cfg.RetryDelayMS)}
	form.fields = make([]textinput.Model, len(values))
	for i, value := range values {
		input := textinput.New()
		input.Prompt = "  "
		input.CharLimit = formInputCharLimit
		input.SetValue(value)
		form.fields[i] = input
	}
	form.focus = 0
	return form.fields[listenAddressFieldIndex].Focus()
}

func (form *Form) configuration() (config.Config, error) {
	cfg := form.base
	cfg.ListenAddress = strings.TrimSpace(form.fields[listenAddressFieldIndex].Value())
	cfg.Distribution = form.fields[distributionFieldIndex].Value()
	cfg.ManualTargetIP = strings.TrimSpace(form.fields[manualTargetIPFieldIndex].Value())
	for _, field := range []struct {
		index  int
		target *int
	}{
		{listenPortFieldIndex, &cfg.ListenPort},
		{targetPortFieldIndex, &cfg.TargetPort},
		{connectTimeoutFieldIndex, &cfg.ConnectTimeoutMS},
		{connectRetriesFieldIndex, &cfg.ConnectRetries},
		{retryDelayFieldIndex, &cfg.RetryDelayMS},
	} {
		value, err := strconv.Atoi(strings.TrimSpace(form.fields[field.index].Value()))
		if err != nil {
			return cfg, fmt.Errorf("%s must be an integer", fieldLabels[field.index])
		}
		*field.target = value
	}
	return cfg, cfg.Validate()
}

func (form *Form) SelectDistribution(name string) tea.Cmd {
	form.fields[distributionFieldIndex].SetValue(name)
	return form.fields[form.focus].Focus()
}

func (form *Form) Help() string { return "Tab/Up/Down: field   Ctrl+S: save   Esc: discard" }

func (form *Form) Resize(area layout.Size, _ Snapshot) {
	form.area = area
	form.resizeInputs()
}

// resizeInputs updates the widget's horizontal scroll window before input events.
func (form *Form) resizeInputs() {
	area := form.area
	for index := range form.fields {
		input := &form.fields[index]
		style := input.Styles().Focused.Text
		frameWidth, _ := style.GetFrameSize()
		// Textinput renders a cursor cell in addition to its text width.
		input.SetWidth(max(1, area.Width-lipgloss.Width(input.Prompt)-frameWidth-1))
		// SetWidth alone does not recompute the horizontal scroll window.
		input.SetCursor(input.Position())
	}
}

func (form *Form) Update(msg tea.Msg) tea.Cmd {
	if len(form.fields) == 0 {
		return nil
	}
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+s":
			cfg, err := form.configuration()
			if err != nil {
				return form.receiver.ShowError(err)
			}
			return form.receiver.SaveConfig(cfg)
		case "enter":
			if form.focus == distributionFieldIndex {
				return form.receiver.ChooseDistribution(true)
			}
			fallthrough
		case "tab", "down":
			form.fields[form.focus].Blur()
			form.focus = (form.focus + 1) % len(form.fields)
			return form.fields[form.focus].Focus()
		case "shift+tab", "up":
			form.fields[form.focus].Blur()
			form.focus = (form.focus + len(form.fields) - 1) % len(form.fields)
			return form.fields[form.focus].Focus()
		}
	}
	var cmd tea.Cmd
	form.fields[form.focus], cmd = form.fields[form.focus].Update(msg)
	return cmd
}

var formTemplate = template.Must(template.New("form").Parse(`{{if .Fields}}{{.Fields}}
{{end}}{{if .Counter}}{{.Counter}}
{{end}}{{define "field"}}{{if .Label}}{{.Label}}
{{end}}{{.Input}}{{end}}{{define "fields"}}{{range $index, $field := .}}{{if $index}}
{{end}}{{$field}}{{end}}{{end}}`))

type formFieldViewData struct{ Label, Input string }
type formViewData struct {
	Fields, Counter string
	fieldValues     []formFieldViewData
}
type formLayout struct {
	fieldsArea, counterArea layout.Size
	fieldBlocks             []string
	firstField, lastField   int
}

func (form *Form) prepareViewData() formViewData {
	fields := make([]formFieldViewData, len(form.fields))
	for index := range form.fields {
		label := muted.Render(fieldLabels[index])
		if index == form.focus {
			label = accent.Render(fieldLabels[index])
		}
		fields[index] = formFieldViewData{Label: label, Input: form.fields[index].View()}
	}
	counter := muted.Render(fmt.Sprintf("Field %d/%d", form.focus+1, len(form.fields)))
	return formViewData{fieldValues: fields, Counter: counter}
}

// calculateLayout admits complete neighboring fields after the focused field.
// It only measures rendered content; it does not change focus or widget state.
func (form *Form) calculateLayout(data formViewData) formLayout {
	fields, counter := data.fieldValues, data.Counter
	if form.area.Height <= 1 {
		counter = ""
	}
	areas := layout.Column(form.area, layout.Block{Grow: true}, layout.Block{Text: counter})
	result := formLayout{fieldsArea: areas[0], counterArea: areas[1], firstField: form.focus, lastField: form.focus + 1}
	result.fieldBlocks = make([]string, len(fields))
	heights := make([]int, len(fields))
	for index, field := range fields {
		_, labelSize := layout.Measure(field.Label, result.fieldsArea.Width)
		labelArea := layout.Size{Width: result.fieldsArea.Width, Height: min(labelSize.Height, max(0, result.fieldsArea.Height-1))}
		field.Label = layout.Fit(field.Label, labelArea)
		var content bytes.Buffer
		if err := formTemplate.ExecuteTemplate(&content, "field", field); err != nil {
			panic("render form field template: " + err.Error())
		}
		result.fieldBlocks[index], _ = layout.Measure(content.String(), result.fieldsArea.Width)
		heights[index] = lipgloss.Height(result.fieldBlocks[index])
	}
	usedHeight := heights[form.focus]
	for result.firstField > 0 && usedHeight+heights[result.firstField-1] <= result.fieldsArea.Height {
		result.firstField--
		usedHeight += heights[result.firstField]
	}
	for result.lastField < len(fields) && usedHeight+heights[result.lastField] <= result.fieldsArea.Height {
		usedHeight += heights[result.lastField]
		result.lastField++
	}
	return result
}

func (form *Form) View(Snapshot) Content {
	if len(form.fields) == 0 {
		return Content{}
	}
	data := form.prepareViewData()
	areas := form.calculateLayout(data)
	var visibleFields bytes.Buffer
	if err := formTemplate.ExecuteTemplate(&visibleFields, "fields", areas.fieldBlocks[areas.firstField:areas.lastField]); err != nil {
		panic("render form fields template: " + err.Error())
	}
	data.Fields = layout.Fit(visibleFields.String(), areas.fieldsArea)
	data.Counter = layout.Fit(data.Counter, areas.counterArea)
	var content bytes.Buffer
	if err := formTemplate.Execute(&content, data); err != nil {
		panic("render form template: " + err.Error())
	}
	return Content{Body: layout.Fit(strings.TrimSuffix(content.String(), "\n"), form.area)}
}
