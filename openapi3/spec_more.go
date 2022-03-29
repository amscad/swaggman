package openapi3

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/gocharts/v2/data/histogram"
	"github.com/grokify/gocharts/v2/data/table"
	"github.com/grokify/gocharts/v2/data/table/tabulator"
	"github.com/grokify/mogo/encoding/jsonutil"
	"github.com/grokify/mogo/net/urlutil"
	"github.com/grokify/mogo/type/stringsutil"
)

type Spec = oas3.T

type Operation = oas3.Operation

type SpecMore struct {
	Spec *Spec
}

func ReadSpecMore(path string, validate bool) (*SpecMore, error) {
	spec, err := ReadFile(path, validate)
	if err != nil {
		return nil, err
	}
	return &SpecMore{Spec: spec}, nil
}

func (sm *SpecMore) SchemasCount() int {
	if sm.Spec == nil {
		return -1
	} else if sm.Spec.Components.Schemas == nil {
		return 0
	}
	return len(sm.Spec.Components.Schemas)
}

func (sm *SpecMore) OperationsTable(columns *tabulator.ColumnSet, filterFunc func(path, method string, op *oas3.Operation) bool) (*table.Table, error) {
	return operationsTable(sm.Spec, columns, filterFunc)
}

func operationsTable(spec *Spec, columns *tabulator.ColumnSet, filterFunc func(path, method string, op *oas3.Operation) bool) (*table.Table, error) {
	if columns == nil {
		columns = OpTableColumnsDefault(false)
	}
	tbl := table.NewTable(spec.Info.Title)
	tbl.Columns = columns.DisplayTexts()

	specMore := SpecMore{Spec: spec}

	tgs, err := specMore.TagGroups()
	if err != nil {
		return nil, err
	}

	VisitOperations(spec, func(path, method string, op *oas3.Operation) {
		if filterFunc != nil &&
			!filterFunc(path, method, op) {
			return
		}
		row := []string{}

		for _, text := range columns.Columns {
			switch text.Slug {
			case "tags":
				row = append(row, strings.Join(op.Tags, ", "))
			case "method":
				row = append(row, method)
			case "path":
				row = append(row, path)
			case "operationId":
				row = append(row, op.OperationID)
			case "summary":
				row = append(row, op.Summary)
			case XTagGroups:
				row = append(row, strings.Join(
					tgs.GetTagGroupNamesForTagNames(op.Tags...), ", "))
			case "securityScopes":
				row = append(row, strings.Join(OperationSecurityScopes(op, false), ", "))
			case XThrottlingGroup:
				row = append(row, GetExtensionPropStringOrEmpty(op.ExtensionProps, XThrottlingGroup))
			case "docsURL":
				if op.ExternalDocs != nil {
					row = append(row, op.ExternalDocs.URL)
				}
			default:
				row = append(row, GetExtensionPropStringOrEmpty(op.ExtensionProps, text.Slug))
			}
		}

		tbl.Rows = append(tbl.Rows, row)
	})
	return &tbl, nil
}

func OpTableColumnsDefault(inclDocsURL bool) *tabulator.ColumnSet {
	cols := []tabulator.Column{
		{
			Display: "Tags",
			Slug:    "tags",
			Width:   150},
		{
			Display: "Method",
			Slug:    "method",
			Width:   70},
		{
			Display: "Path",
			Slug:    "path",
			Width:   800},
		{
			Display: "OperationID",
			Slug:    "operationId",
			Width:   150},
		{
			Display: "Summary",
			Slug:    "summary",
			Width:   150},
		{
			Display: "SecurityScopes",
			Slug:    "securityScopes",
			Width:   150},
		{
			Display: "XThrottlingGroup",
			Slug:    XThrottlingGroup,
			Width:   150},
	}
	if inclDocsURL {
		cols = append(cols, tabulator.Column{
			Display: "DocsURL",
			Slug:    "docsURL",
			Width:   150})
	}
	return &tabulator.ColumnSet{Columns: cols}
}

func OpTableColumnsRingCentral() *tabulator.ColumnSet {
	columns := OpTableColumnsDefault(false)
	rcCols := []tabulator.Column{
		{
			Display: "API Group",
			Slug:    "x-api-group",
			Width:   150},
		{
			Display: "Throttling",
			Slug:    "x-throttling-group",
			Width:   150},
		{
			Display: "App Permission",
			Slug:    "x-app-permission",
			Width:   150},
		{
			Display: "User Permissions",
			Slug:    "x-user-permission",
			Width:   150},
	}
	columns.Columns = append(columns.Columns, rcCols...)
	return columns
	//return &table.ColumnSet{Columns: columns}
}

func (sm *SpecMore) OperationMetas() []OperationMeta {
	ometas := []OperationMeta{}
	if sm.Spec == nil {
		return ometas
	}
	for url, path := range sm.Spec.Paths {
		if path.Connect != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodConnect, path.Connect))
		}
		if path.Delete != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodDelete, path.Delete))
		}
		if path.Get != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodGet, path.Get))
		}
		if path.Head != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodHead, path.Head))
		}
		if path.Options != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodOptions, path.Options))
		}
		if path.Patch != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodPatch, path.Patch))
		}
		if path.Post != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodPost, path.Post))
		}
		if path.Put != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodPut, path.Put))
		}
		if path.Trace != nil {
			ometas = append(ometas, OperationToMeta(url, http.MethodTrace, path.Trace))
		}
	}

	return ometas
}

func (sm *SpecMore) OperationsCount() int {
	if sm.Spec == nil {
		return -1
	}
	return len(sm.OperationMetas())
}

// OperationCountsByTag returns a histogram for operations by tag.
func (sm *SpecMore) OperationCountsByTag() *histogram.Histogram {
	hist := histogram.NewHistogram("Operation Counts by Tag")
	hist.Bins = sm.TagsMap(false, true)
	hist.Inflate()
	return hist
}

func (sm *SpecMore) OperationsIDs() []string {
	ids := []string{}
	VisitOperations(sm.Spec, func(thisPath, thisMethod string, thisOp *oas3.Operation) {
		if thisOp == nil {
			return
		}
		ids = append(ids, thisOp.OperationID)
	})
	ids = stringsutil.SliceCondenseSpace(ids, false, true)
	return ids
}

func (sm *SpecMore) OperationByID(wantOperationID string) (path, method string, op *oas3.Operation, err error) {
	wantOperationID = strings.TrimSpace(wantOperationID)
	VisitOperations(sm.Spec, func(thisPath, thisMethod string, thisOp *oas3.Operation) {
		if thisOp == nil {
			return
		}
		if wantOperationID == strings.TrimSpace(thisOp.OperationID) {
			path = thisPath
			method = thisMethod
			op = thisOp
		}
	})
	if len(strings.TrimSpace(method)) == 0 {
		err = fmt.Errorf("operation_not_found [%s]", wantOperationID)
	}
	return path, method, op, err
}

func (sm *SpecMore) SetOperation(path, method string, op *oas3.Operation) {
	path = strings.TrimSpace(path)
	if strings.Index(path, "/") != 0 {
		path = "/" + path
	}
	if sm.Spec.Paths == nil {
		sm.Spec.Paths = map[string]*oas3.PathItem{}
	}
	pathItem, ok := sm.Spec.Paths[path]
	if !ok {
		pathItem = &oas3.PathItem{}
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case http.MethodConnect:
		pathItem.Connect = op
	case http.MethodDelete:
		pathItem.Delete = op
	case http.MethodGet:
		pathItem.Get = op
	case http.MethodHead:
		pathItem.Head = op
	case http.MethodOptions:
		pathItem.Options = op
	case http.MethodPatch:
		pathItem.Patch = op
	case http.MethodPost:
		pathItem.Post = op
	case http.MethodPut:
		pathItem.Put = op
	case http.MethodTrace:
		pathItem.Trace = op
	}
	sm.Spec.Paths[path] = pathItem
}

func (sm *SpecMore) SchemaNames() []string {
	schemaNames := []string{}
	for schemaName := range sm.Spec.Components.Schemas {
		schemaNames = append(schemaNames, schemaName)
	}
	return stringsutil.SliceCondenseSpace(schemaNames, true, true)
}

var rxSchemas = regexp.MustCompile(`"([^"]*#/components/schemas/([^"]+))"`)

func (sm *SpecMore) SchemaPointers(dedupe bool) ([]string, []string, error) {
	bytes, err := sm.MarshalJSON("", "")
	if err != nil {
		return []string{}, []string{}, err
	}
	pointers := []string{}
	names := []string{}
	m := rxSchemas.FindAllStringSubmatch(string(bytes), -1)
	for _, mx := range m {
		if len(mx) == 3 {
			pointers = append(pointers, mx[1])
			names = append(names, mx[2])
		}
	}
	return stringsutil.SliceCondenseSpace(pointers, dedupe, true),
		stringsutil.SliceCondenseSpace(names, dedupe, true),
		nil
}

func (sm *SpecMore) SchemaNamesStatus() (schemaNoReference, both, referenceNoSchema []string, err error) {
	haveNames := sm.SchemaNames()
	_, havePointers, err := sm.SchemaPointers(true)
	if err != nil {
		return
	}
	schemaNoReference, both, referenceNoSchema = stringsutil.SlicesCompare(haveNames, havePointers)
	return
}

func (sm *SpecMore) SchemaNameExists(schemaName string, includeNil bool) bool {
	for schemaNameTry, schemaRef := range sm.Spec.Components.Schemas {
		if schemaNameTry == schemaName {
			if includeNil {
				return true
			} else if schemaRef == nil {
				return false
			}
			schemaRef.Ref = strings.TrimSpace(schemaRef.Ref)
			if len(schemaRef.Ref) > 0 {
				return true
			}
			if schemaRef.Value == nil {
				return false
			} else {
				return true
			}
		}
	}
	return false
}

func (sm *SpecMore) SchemaRef(schemaName string) *oas3.SchemaRef {
	for schemaNameTry, schemaRef := range sm.Spec.Components.Schemas {
		if schemaName == schemaNameTry {
			return schemaRef
		}
	}
	return nil
}

func (sm *SpecMore) SetSchemaRef(schemaName string, schemaRef *oas3.SchemaRef) error {
	schemaName = strings.TrimSpace(schemaName)
	if schemaRef != nil {
		if sm.Spec.Components.Schemas == nil {
			sm.Spec.Components.Schemas = map[string]*oas3.SchemaRef{}
		}
		if schemaRef.Value != nil {
			if 1 == 0 && len(schemaRef.Value.Description) == 0 {
				return fmt.Errorf("no description for schema component [%s]", schemaName)
			}
		}
	}
	sm.Spec.Components.Schemas[schemaName] = schemaRef
	return nil
}

// ServerURL returns the OAS3 Spec URL for the index
// specified.
func (sm *SpecMore) ServerURL(index uint) string {
	if int(index)+1 > len(sm.Spec.Servers) {
		return ""
	}
	server := sm.Spec.Servers[index]
	return strings.TrimSpace(server.URL)
}

// ServerURLBasePath extracts the base path from a OAS URL
// which can include variables.
func (sm *SpecMore) ServerURLBasePath(index uint) (string, error) {
	serverURL := sm.ServerURL(index)
	if len(serverURL) == 0 {
		return "", nil
	}
	serverURLParsed, err := urlutil.ParseURLTemplate(serverURL)
	if err != nil {
		return "", err
	}
	return serverURLParsed.Path, nil
}

func (sm *SpecMore) Tags(inclTop, inclOps bool) []string {
	tags := []string{}
	tagsMap := sm.TagsMap(inclTop, inclOps)
	for tag := range tagsMap {
		tags = append(tags, tag)
	}
	return stringsutil.SliceCondenseSpace(tags, true, true)
}

// TagsMap returns a set of tags present in the current spec.
func (sm *SpecMore) TagsMap(inclTop, inclOps bool) map[string]int {
	tagsMap := map[string]int{}
	if inclTop {
		for _, tag := range sm.Spec.Tags {
			tagName := strings.TrimSpace(tag.Name)
			if len(tagName) > 0 {
				if _, ok := tagsMap[tagName]; !ok {
					tagsMap[tagName] = 0
				}
				tagsMap[tagName]++
			}
		}
	}
	if inclOps {
		VisitOperations(sm.Spec, func(skipPath, skipMethod string, op *oas3.Operation) {
			for _, tagName := range op.Tags {
				tagName = strings.TrimSpace(tagName)
				if len(tagName) > 0 {
					if _, ok := tagsMap[tagName]; !ok {
						tagsMap[tagName] = 0
					}
					tagsMap[tagName]++
				}
			}
		})
	}
	return tagsMap
}

type SpecStats struct {
	OperationsCount int
	SchemasCount    int
}

func (sm *SpecMore) Stats() SpecStats {
	return SpecStats{
		OperationsCount: sm.OperationsCount(),
		SchemasCount:    sm.SchemasCount(),
	}
}

func (sm *SpecMore) MarshalJSON(prefix, indent string) ([]byte, error) {
	bytes, err := sm.Spec.MarshalJSON()
	if err != nil {
		return bytes, err
	}
	pretty := false
	if len(prefix) > 0 || len(indent) > 0 {
		pretty = true
	}
	if pretty {
		bytes = jsonutil.PrettyPrint(bytes, "", "  ")
	}
	return bytes, nil
}

func (sm *SpecMore) PrintJSON(prefix, indent string) error {
	bytes, err := sm.MarshalJSON(prefix, indent)
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(bytes))
	return err
}

func (sm *SpecMore) WriteFileCSV(filename string) error {
	tbl, err := sm.OperationsTable(nil, nil)
	if err != nil {
		return err
	}
	return tbl.WriteCSV(filename)
}

func (sm *SpecMore) WriteFileJSON(filename string, perm os.FileMode, prefix, indent string) error {
	jsonData, err := sm.MarshalJSON(prefix, indent)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, jsonData, perm)
}

func (sm *SpecMore) WriteFileXLSX(filename string, columns *tabulator.ColumnSet, filterFunc func(path, method string, op *oas3.Operation) bool) error {
	if columns == nil {
		columns = OpTableColumnsDefault(true)
	}
	tbl, err := sm.OperationsTable(columns, filterFunc)
	if err != nil {
		return err
	}
	tbl.FormatAutoLink = true
	return table.WriteXLSX(filename, tbl)
}

type TagsMore struct {
	Tags oas3.Tags
}

func (tg *TagsMore) Get(tagName string) *oas3.Tag {
	for _, tag := range tg.Tags {
		if tagName == tag.Name {
			return tag
		}
	}
	return nil
}
