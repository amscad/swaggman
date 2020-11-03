package topostman2

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"strings"

	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/swaggman/openapi3"
	"github.com/grokify/swaggman/postman2"
	"github.com/grokify/swaggman/postman2/simple"
)

//const DefaultContentTypePreferences string = `multipart/form-data,application/json,application/x-www-form-urlencoded,application/xml,text/plain`
//var defaultContentTypePreferencesSlice = strings.Split(DefaultContentTypePreferences, ",")

// Configuration is a Swaggman configuration that holds information on how
// to create the Postman 2.0 collection including overriding Swagger 2.0
// spec values.
type Configuration struct {
	PostmanServerURLBasePath string            `json:"postmanServerUrlApiBasePath,omitempty"`
	PostmanServerURL         string            `json:"postmanServerUrl,omitempty"`
	PostmanURLHostname       string            `json:"postmanURLHostname,omitempty"`
	PostmanHeaders           []postman2.Header `json:"postmanHeaders,omitempty"`
	UseXTagGroups            bool              `json:"useXTagGroups,omitempty"`
}

// Converter is the struct that manages the conversion.
type Converter struct {
	Configuration Configuration
	Swagger       *oas3.Swagger
}

// NewConverter instantiates a new converter.
func NewConverter(cfg Configuration) Converter {
	return Converter{Configuration: cfg}
}

// MergeConvert builds a Postman 2.0 spec using a base Postman 2.0 collection
// and a Swagger 2.0 spec.
func (conv *Converter) MergeConvert(openapiFilepath string, pmanBaseFilepath string, pmanSpecFilepath string) error {
	/*oas3Loader := oas3.NewSwaggerLoader()
	oas3spec, err := oas3Loader.LoadSwaggerFromFile(openapiFilepath)
	if err != nil {
		return err
	}*/
	oas3spec, err := openapi3.ReadFile(openapiFilepath, true)
	if err != nil {
		return err
	}

	pmanBaseFilepath = strings.TrimSpace(pmanBaseFilepath)
	if len(pmanBaseFilepath) > 0 {
		pman, err := simple.ReadCanonicalCollection(pmanBaseFilepath)
		if err != nil {
			return err
		}

		pm, err := Merge(conv.Configuration, pman, oas3spec)
		if err != nil {
			return err
		}

		bytes, err := json.MarshalIndent(pm, "", "  ")
		if err != nil {
			return err
		}
		return ioutil.WriteFile(pmanSpecFilepath, bytes, 0644)
	}
	return conv.Convert(openapiFilepath, pmanSpecFilepath)
}

// Convert builds a Postman 2.0 spec using a Swagger 2.0 spec.
func (conv *Converter) Convert(openapiFilepath string, pmanSpecFilepath string) error {
	//swag, err := swagger2.ReadSwagger2Spec(openapiFilepath)
	oas3Loader := oas3.NewSwaggerLoader()
	oas3spec, err := oas3Loader.LoadSwaggerFromFile(openapiFilepath)
	if err != nil {
		return err
	}
	pm, err := Convert(conv.Configuration, oas3spec)
	if err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(pm, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pmanSpecFilepath, bytes, 0644)
}

// Convert creates a Postman 2.0 collection from a configuration and Swagger 2.0 spec
func Convert(cfg Configuration, oas3spec *oas3.Swagger) (postman2.Collection, error) {
	return Merge(cfg, postman2.Collection{}, oas3spec)
}

// Merge creates a Postman 2.0 collection from a configuration, base Postman
// 2.0 collection and Swagger 2.0 spec
func Merge(cfg Configuration, pman postman2.Collection, oas3spec *oas3.Swagger) (postman2.Collection, error) {
	if len(pman.Info.Name) == 0 {
		pman.Info.Name = strings.TrimSpace(oas3spec.Info.Title)
	}
	if len(pman.Info.Description) == 0 {
		pman.Info.Description = strings.TrimSpace(oas3spec.Info.Description)
	}
	if len(pman.Info.Schema) == 0 {
		pman.Info.Schema = "https://schema.getpostman.com/json/collection/v2.0.0/collection.json"
	}

	pman, err := CreateTagsAndTagGroups(pman, oas3spec)
	if err != nil {
		return pman, err
	}

	oas3specMore := openapi3.SpecMore{Spec: oas3spec}
	tagGroupSet, err := oas3specMore.TagGroups()
	//tagGroupSet, err := openapi3.SpecTagGroups(oas3spec)
	if err != nil {
		return pman, err
	}

	urls := []string{}
	for url := range oas3spec.Paths {
		urls = append(urls, url)
	}
	sort.Strings(urls)

	for _, url := range urls {
		path := oas3spec.Paths[url] // *PathItem

		if path.Get != nil {
			pman = postmanAddItemToFolders(pman,
				Openapi3OperationToPostman2APIItem(cfg, oas3spec, url, http.MethodGet, path.Get),
				path.Get.Tags, tagGroupSet)
		}
		if path.Patch != nil {
			pman = postmanAddItemToFolders(pman,
				Openapi3OperationToPostman2APIItem(cfg, oas3spec, url, http.MethodPatch, path.Patch),
				path.Patch.Tags, tagGroupSet)
		}
		if path.Post != nil {
			pman = postmanAddItemToFolders(pman,
				Openapi3OperationToPostman2APIItem(cfg, oas3spec, url, http.MethodPost, path.Post),
				path.Post.Tags, tagGroupSet)
		}
		if path.Put != nil {
			pman = postmanAddItemToFolders(pman,
				Openapi3OperationToPostman2APIItem(cfg, oas3spec, url, http.MethodPut, path.Put),
				path.Put.Tags, tagGroupSet)
		}
		if path.Delete != nil {
			pman = postmanAddItemToFolders(pman,
				Openapi3OperationToPostman2APIItem(cfg, oas3spec, url, http.MethodDelete, path.Delete),
				path.Delete.Tags, tagGroupSet)
		}
	}

	return pman, nil
}

func postmanAddItemToFolders(pman postman2.Collection, pmItem *postman2.Item, tagNames []string, tagGroupSet openapi3.TagGroupSet) postman2.Collection {
	for _, tagName := range tagNames {
		tagGroupNames := tagGroupSet.GetTagGroupNamesForTagNames(tagName)
		if len(tagGroupNames) == 0 {
			pmFolder := pman.GetOrNewFolder(tagName)
			pmFolder.Item = append(pmFolder.Item, pmItem)
			pman.SetFolder(pmFolder)
		} else {
			for _, tagGroupName := range tagGroupNames {
				pmFolder := pman.GetOrNewFolder(tagGroupName)
				if pmFolder.Item == nil {
					pmFolder.Item = []*postman2.Item{}
				}
				// Tags
				modded := false
				for i, pmfSubItem := range pmFolder.Item {
					if pmfSubItem.Name == tagName {
						if pmfSubItem.Item == nil {
							if pmfSubItem.Item == nil {
								pmfSubItem.Item = []*postman2.Item{}
							}
						}
						pmfSubItem.Item = append(pmfSubItem.Item, pmItem)
						pmFolder.Item[i] = pmfSubItem
						modded = true
					}
				}
				if modded {
					pman.SetFolder(pmFolder)
				}
			}
		}
	}

	return pman
}

func postmanAddItemToFolder(pman postman2.Collection, pmItem *postman2.Item, pmFolderName string) postman2.Collection {
	pmFolder := pman.GetOrNewFolder(pmFolderName)
	pmFolder.Item = append(pmFolder.Item, pmItem)
	pman.SetFolder(pmFolder)
	return pman
}

func Openapi3OperationToPostman2APIItem(cfg Configuration, oas3spec *oas3.Swagger, url string, method string, operation *oas3.Operation) *postman2.Item {
	pmUrl := BuildPostmanURL(cfg, oas3spec, url, operation)
	item := &postman2.Item{
		Name: operation.Summary,
		Request: &postman2.Request{
			Method: strings.ToUpper(method),
			URL:    &pmUrl,
		},
	}

	headers := cfg.PostmanHeaders

	headers, _, _ = postman2.AddOperationReqResMediaTypeHeaders(
		headers, operation,
		postman2.DefaultMediaTypePreferencesSlice(),
		postman2.DefaultMediaTypePreferencesSlice(),
	)

	item.Request.Header = headers

	params := ParamsOpenAPI3ToPostman(operation.Parameters)
	if len(params.Query) > 0 {
		item.Request.URL.Query = params.Query
	}
	if len(params.Variable) > 0 {
		item.Request.URL.Variable = params.Variable
	}

	return item
}

func BuildPostmanURL(cfg Configuration, spec *oas3.Swagger, specPath string, operation *oas3.Operation) postman2.URL {
	specMore := openapi3.SpecMore{Spec: spec}
	specServerURL := specMore.ServerURL(0)
	overrideServerURL := cfg.PostmanServerURL
	specURLString := openapi3.BuildApiUrlOAS(specServerURL, overrideServerURL, specPath)
	pmanURLString := postman2.ApiUrlOasToPostman(specURLString)
	pmanURL := postman2.NewURL(pmanURLString)
	pmanURL = PostmanUrlAddDefaultsOAS3(pmanURL, operation)
	return pmanURL
}

var postmanUrlDefaultsRx *regexp.Regexp = regexp.MustCompile(`^\s*(:(.+))\s*$`)

func PostmanUrlAddDefaultsOAS3(pmanURL postman2.URL, operation *oas3.Operation) postman2.URL {
	for _, part := range pmanURL.Path {
		match := postmanUrlDefaultsRx.FindAllStringSubmatch(part, -1)
		if len(match) > 0 {
			baseVariable := match[0][2]
			var defaultValue interface{}
			for _, parameterRef := range operation.Parameters {
				if parameterRef == nil || parameterRef.Value == nil {
					continue
				}
				if parameterRef.Value.Name != baseVariable {
					continue
				}
				schemaRef := parameterRef.Value.Schema
				if schemaRef == nil || schemaRef.Value == nil {
					continue
				}
				if schemaRef.Value.Default != nil {
					defaultValue = schemaRef.Value.Default
					pmanURL.AddVariable(baseVariable, defaultValue)
				}
				/*
					if parameter.Name == baseVariable {
						defaultValue = parameter.Default
						break
					}*/
			}
			//pmanURL.AddVariable(baseVariable, defaultValue)
		}
	}
	return pmanURL
}

// ParamsOpenAPI3ToPostman returns a slices of Postman parameters
// for a slice of OpenAPI 3 parameters.
func ParamsOpenAPI3ToPostman(oparams []*oas3.ParameterRef) postman2.URLParameters {
	pparams := postman2.NewURLParameters()
	for _, oparamRef := range oparams {
		if oparamRef == nil || oparamRef.Value == nil {
			continue
		}
		oparam := oparamRef.Value
		if oparam.In == oas3.ParameterInQuery {
			pparams.Query = append(pparams.Query,
				postman2.URLQuery{
					Key:         oparam.Name,
					Value:       schemaToString(oparam.Schema),
					Description: oparam.Description,
					Disabled:    true,
				},
			)
		}
	}
	return pparams
}

func schemaToString(schemaRef *oas3.SchemaRef) string {
	if schemaRef == nil || schemaRef.Value == nil {
		return ""
	}
	schema := schemaRef.Value
	parts := []string{}
	schema.Type = strings.TrimSpace(schema.Type)
	schema.Format = strings.TrimSpace(schema.Format)
	if len(schema.Type) > 0 {
		parts = append(parts, schema.Type)
	}
	if len(schema.Format) > 0 {
		parts = append(parts, schema.Format)
	}
	if strings.ToLower(schema.Type) == "array" {
		if schema.Items != nil && schema.Items.Value != nil {
			parts = append(parts, schema.Items.Value.Type)
		}
	}
	if len(parts) > 0 {
		if len(parts) == 2 && parts[0] == "array" && parts[1] == "string" {
			parts = append(parts, "csv")
		}
		return "<" + strings.Join(parts, ".") + ">"
	} else {
		return ""
	}
}
