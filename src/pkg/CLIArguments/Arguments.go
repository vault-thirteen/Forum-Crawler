package cli

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/vault-thirteen/auxie/number"
)

const (
	ActionInit    = "init"
	ActionRefresh = "refresh"
	ActionUpdate  = "update"
)

const (
	ObjectForums      = "forums"
	ObjectForumTopics = "forum_topics"
	ObjectAllTopics   = "all_topics"
)

const (
	ErrUnknownAction       = "unknown action: %v"
	ErrUnsupportedObject   = "unsupported object: %v"
	ErrArgs                = "error in command line arguments"
	ErrBadParameter        = "bad parameter: %v"
	ErrParameterIsNotFound = "parameter is not found: %v"
)

const (
	NoParameters               = "-"
	ParameterPairsSeparator    = ","
	ParameterKeyValueSeparator = "="
)

const (
	Parameter_ForumId      = "forum_id"
	Parameter_StartForumId = "start_forum_id"
	Parameter_ForumPage    = "forum_page"
	Parameter_FirstPages   = "first_pages"
)

type Arguments struct {
	SettingsFile  string
	Action        string
	Object        string
	Parameters    []Parameter
	parametersRaw string
}

func NewArguments() (args *Arguments, err error) {
	args, err = getRawArgumentsFromOs()
	if err != nil {
		return nil, err
	}

	args.Parameters, err = parseRawArguments(args.parametersRaw)
	if err != nil {
		return nil, err
	}

	return args, nil
}

func getRawArgumentsFromOs() (args *Arguments, err error) {
	tpl := Arguments{}
	expectedArgsCount := tpl.ExportedFieldsCount() + 1

	if len(os.Args) != expectedArgsCount {
		fmt.Println(tpl.UsageText())
		return nil, errors.New(ErrArgs)
	}

	args = &Arguments{}

	args.SettingsFile = os.Args[1]
	args.Action = os.Args[2]
	args.Object = os.Args[3]
	args.parametersRaw = os.Args[4]

	return args, nil
}

func parseRawArguments(rawParameters string) (parameters []Parameter, err error) {
	if rawParameters == NoParameters {
		return nil, nil
	}

	parameters = make([]Parameter, 0)

	parts := strings.Split(rawParameters, ParameterPairsSeparator)

	for _, part := range parts {
		kvs := strings.Split(part, ParameterKeyValueSeparator)
		if len(kvs) != 2 {
			return nil, fmt.Errorf(ErrBadParameter, part)
		}
		parameters = append(parameters, Parameter{Key: kvs[0], Value: kvs[1]})
	}

	return parameters, nil
}

func (a *Arguments) ExportedFieldsCount() int {
	exportedFieldsCount := 0
	fieldsCount := reflect.TypeOf(Arguments{}).NumField()

	for i := 0; i < fieldsCount; i++ {
		fieldName := reflect.TypeOf(Arguments{}).Field(i).Name
		if isFirstLetterCapital(fieldName) {
			exportedFieldsCount++
		}
	}

	return exportedFieldsCount
}

func (a *Arguments) ExportedFieldNames() (efnames []string) {
	fieldsCount := reflect.TypeOf(Arguments{}).NumField()
	efnames = make([]string, 0, fieldsCount)

	for i := 0; i < fieldsCount; i++ {
		fieldName := reflect.TypeOf(Arguments{}).Field(i).Name
		if isFirstLetterCapital(fieldName) {
			efnames = append(efnames, fieldName)
		}
	}

	return efnames
}

func isFirstLetterCapital(s string) bool {
	letters := []rune(s)
	if len(letters) == 0 {
		return false
	}

	firstLetter := string(letters[0])
	firstLetterCap := strings.ToUpper(firstLetter)

	return firstLetter == firstLetterCap
}

func (a *Arguments) UsageText() string {
	var sb strings.Builder
	sb.WriteString("Usage: program.exe ")

	fieldNames := a.ExportedFieldNames()
	for _, fname := range fieldNames {
		sb.WriteString(fmt.Sprintf("[%v] ", fname))
	}

	return sb.String()
}

func (a *Arguments) GetForumId() (fid uint, err error) {
	return a.getNamedParameterValueAsUint(Parameter_ForumId)
}

func (a *Arguments) GetStartForumId() (fid uint, err error) {
	return a.getNamedParameterValueAsUint(Parameter_StartForumId)
}

func (a *Arguments) GetFirstPages() (fid uint, err error) {
	return a.getNamedParameterValueAsUint(Parameter_FirstPages)
}

func (a *Arguments) GetForumPage() (fp uint, err error) {
	return a.getNamedParameterValueAsUint(Parameter_ForumPage)
}

func (a *Arguments) getNamedParameterValueAsUint(name string) (param uint, err error) {
	var p *Parameter
	p, err = a.getNamedParameter(name)
	if err != nil {
		return 0, err
	}

	return number.ParseUint(p.Value)
}

func (a *Arguments) getNamedParameter(name string) (parameter *Parameter, err error) {
	for _, p := range a.Parameters {
		if p.Key == name {
			return &p, nil
		}
	}

	return nil, fmt.Errorf(ErrParameterIsNotFound, name)
}
