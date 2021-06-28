package openapi3edit

import (
	"regexp"

	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/spectrum/openapi3"
)

func SpecSchemasSetDeprecated(spec *openapi3.Spec, newDeprecated bool) {
	for _, schemaRef := range spec.Components.Schemas {
		if len(schemaRef.Ref) == 0 && schemaRef.Value != nil {
			schemaRef.Value.Deprecated = newDeprecated
		}
	}
}

func SpecOperationsSetDeprecated(spec *openapi3.Spec, newDeprecated bool) {
	openapi3.VisitOperations(
		spec,
		func(path, method string, op *oas3.Operation) {
			if op != nil {
				op.Deprecated = newDeprecated
			}
		},
	)
}

var rxDeprecated = regexp.MustCompile(`(?i)\bdeprecated\b`)

func SpecSetDeprecatedImplicit(spec *openapi3.Spec) {
	openapi3.VisitOperations(
		spec,
		func(path, method string, op *oas3.Operation) {
			if op != nil && rxDeprecated.MatchString(op.Description) {
				op.Deprecated = true
			}
		},
	)
	for _, schemaRef := range spec.Components.Schemas {
		if len(schemaRef.Ref) == 0 && schemaRef.Value != nil {
			if rxDeprecated.MatchString(schemaRef.Value.Description) {
				schemaRef.Value.Deprecated = true
			}
			for _, propRef := range schemaRef.Value.Properties {
				if len(propRef.Ref) == 0 && propRef.Value != nil {
					if rxDeprecated.MatchString(propRef.Value.Description) {
						propRef.Value.Deprecated = true
					}
				}
			}
		}
	}
}
