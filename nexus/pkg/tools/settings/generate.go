package generate

import (
	"encoding/json"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"os"
	"strings"
	"text/template"
)

type Attribute struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type TemplateData struct {
	StructName string
	Attributes []Attribute
}

func snakeToCamel(str string) string {
	words := strings.Split(str, "_")
	for i := range words {
		words[i] = cases.Title(language.Und, cases.NoLower).String(words[i])
	}
	return strings.Join(words, "")
}

func getGetValueFunction(attributeType string) string {
	switch attributeType {
	case "string":
		return "GetStringValue"
	case "int64":
		return "GetIntValue"
	case "bool":
		return "GetBoolValue"
	case "float64":
		return "GetFloatValue"
	default:
		panic(fmt.Sprintf("Unknown attribute type: %s", attributeType))
	}
}

func readAttributesFromFile(filePath string) (TemplateData, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return TemplateData{}, err
	}

	var data TemplateData
	err = json.Unmarshal(file, &data)
	if err != nil {
		return TemplateData{}, err
	}

	return data, nil
}

func Generate(jsonFilePath, tmplName, outFileName string) {
	data, err := readAttributesFromFile(jsonFilePath)
	if err != nil {
		fmt.Println("Failed to read attributes from file:", err)
		return
	}

	// Create a template with the custom functions
	tmpl := template.New(tmplName).Funcs(template.FuncMap{
		"snakeToCamel":        snakeToCamel,
		"getGetValueFunction": getGetValueFunction,
	})

	// Parse the struct template
	tmpl, err = tmpl.ParseFiles(tmplName)
	if err != nil {
		fmt.Println("Failed to parse struct template:", err)
		return
	}

	outputFile, err := os.Create(outFileName)
	if err != nil {
		fmt.Println("Failed to create output file:", err)
		return
	}
	defer outputFile.Close()

	// Execute the template with the data and write the output to the file
	err = tmpl.Execute(outputFile, data)
	if err != nil {
		fmt.Println("Failed to execute template:", err)
		return
	}

	fmt.Println("Successfully generated output.go")
}
