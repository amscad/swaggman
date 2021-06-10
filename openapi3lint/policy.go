package openapi3lint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	oas3 "github.com/getkin/kin-openapi/openapi3"
	"github.com/grokify/simplego/encoding/jsonutil"
	"github.com/grokify/simplego/log/severity"
	"github.com/grokify/simplego/text/stringcase"
	"github.com/grokify/simplego/type/stringsutil"
	"github.com/grokify/spectrum/openapi3"
	"github.com/grokify/spectrum/openapi3lint/lintutil"
)

type PolicyConfig struct {
	Name             string                `json:"name"`
	Version          string                `json:"version"`
	LastUpdated      time.Time             `json:"lastUpdated,omitempty"`
	Rules            map[string]RuleConfig `json:"rules,omitempty"`
	NonStandardRules []string              `json:"nonStandardRules,omitempty"`
}

func NewPolicyConfigFile(filename string) (PolicyConfig, error) {
	pol := PolicyConfig{}
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return pol, err
	}
	err = json.Unmarshal(bytes, &pol)
	return pol, err
}

func (polCfg *PolicyConfig) RuleNames() ([]string, []string, []string) {
	stdRuleNames := NewStandardRuleNames()
	all := []string{}
	standard := []string{}
	custom := []string{}
	for ruleName := range polCfg.Rules {
		all = append(all, ruleName)
		if stdRuleNames.Exists(ruleName) {
			standard = append(standard, ruleName)
		} else {
			custom = append(custom, ruleName)
		}
	}
	return stringsutil.SliceCondenseSpace(all, true, true),
		stringsutil.SliceCondenseSpace(standard, true, true),
		stringsutil.SliceCondenseSpace(custom, true, true)
}

type RuleConfig struct {
	Severity string `json:"severity"`
}

func (cfg *PolicyConfig) StandardPolicy() (Policy, error) {
	pol := Policy{rules: map[string]Rule{}}
	for ruleName, ruleCfg := range cfg.Rules {
		if err := pol.addRuleWithPriorError(NewStandardRule(ruleName, ruleCfg.Severity)); err != nil {
			return pol, err
		}
	}
	return pol, nil
}

type Policy struct {
	rules map[string]Rule
}

func (pol *Policy) addRuleWithPriorError(rule Rule, err error) error {
	if err != nil {
		return err
	}
	return pol.AddRule(rule, true)
}

func (pol *Policy) AddRule(rule Rule, errorOnCollision bool) error {
	if len(rule.Name()) == 0 {
		return errors.New("rule to add must have non-empty name")
	}
	if !stringcase.IsKebabCase(rule.Name()) {
		return fmt.Errorf("rule to add name must be in in kebab-case format [%s]", rule.Name())
	}
	if _, ok := pol.rules[rule.Name()]; ok {
		if errorOnCollision {
			return fmt.Errorf("add rule collision for [%s]", rule.Name())
		}
	}
	pol.rules[rule.Name()] = rule
	return nil
}

func (pol *Policy) RuleNames() []string {
	ruleNames := []string{}
	for rn := range pol.rules {
		ruleNames = append(ruleNames, rn)
	}
	return ruleNames
}

func (pol *Policy) ValidateSpec(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	vsets := lintutil.NewPolicyViolationsSets()

	unknownScopes := []string{}
	for _, rule := range pol.rules {
		_, err := lintutil.ParseScope(rule.Scope())
		if err != nil {
			unknownScopes = append(unknownScopes, rule.Scope())
		}
	}
	if len(unknownScopes) > 0 {
		return nil, fmt.Errorf("bad policy: rules have unknown scopes [%s]",
			strings.Join(unknownScopes, ","))
	}

	vsetsOps, err := pol.processRulesOperation(spec, pointerBase, filterSeverity)
	if err != nil {
		return vsets, err
	}
	vsets.UpsertSets(vsetsOps)

	vsetsSpec, err := pol.processRulesSpecification(spec, pointerBase, filterSeverity)
	if err != nil {
		return vsets, err
	}
	vsets.UpsertSets(vsetsSpec)

	return vsets, nil
}

func (pol *Policy) processRulesSpecification(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	if spec == nil {
		return nil, errors.New("cannot process nil spec")
	}
	vsets := lintutil.NewPolicyViolationsSets()

	for _, rule := range pol.rules {
		if !lintutil.ScopeMatch(lintutil.ScopeSpecification, rule.Scope()) {
			continue
		}
		inclRule, err := severity.SeverityInclude(filterSeverity, rule.Severity())
		if err != nil {
			return vsets, err
		}
		// fmt.Printf("FILTER_SEV [%v] ITEM_SEV [%v] INCL [%v]\n", filterSeverity, rule.Severity(), inclRule)
		if inclRule {
			//fmt.Printf("PROC RULE name[%s] scope[%s] sev[%s]\n", rule.Name(), rule.Scope(), rule.Severity())
			vsets.AddViolations(rule.ProcessSpec(spec, pointerBase))
		}
	}
	return vsets, nil
}

func (pol *Policy) processRulesOperation(spec *oas3.Swagger, pointerBase, filterSeverity string) (*lintutil.PolicyViolationsSets, error) {
	vsets := lintutil.NewPolicyViolationsSets()

	severityErrorRules := []string{}
	unknownSeverities := []string{}

	openapi3.VisitOperations(spec,
		func(path, method string, op *oas3.Operation) {
			if op == nil {
				return
			}
			opPointer := jsonutil.PointerSubEscapeAll(
				"%s#/paths/%s/%s", pointerBase, path, strings.ToLower(method))
			for _, rule := range pol.rules {
				if !lintutil.ScopeMatch(lintutil.ScopeOperation, rule.Scope()) {
					continue
				}
				//fmt.Printf("HERE [%s] RULE [%s] Scope [%s]\n", path, rule.Name(), rule.Scope())
				inclRule, err := severity.SeverityInclude(filterSeverity, rule.Severity())
				//fmt.Printf("INCL_RULE? [%v] RULE [%s]\n", inclRule, rule.Name())
				if err != nil {
					severityErrorRules = append(severityErrorRules, rule.Name())
					unknownSeverities = append(unknownSeverities, rule.Severity())
				} else if inclRule {
					vsets.AddViolations(rule.ProcessOperation(spec, op, opPointer, path, method))
				}
			}
		},
	)

	if len(severityErrorRules) > 0 || len(unknownSeverities) > 0 {
		severityErrorRules = stringsutil.Dedupe(severityErrorRules)
		sort.Strings(severityErrorRules)
		return vsets, fmt.Errorf(
			"rules with unknown severities rules[%s] severities[%s] valid[%s]",
			strings.Join(unknownSeverities, ","),
			strings.Join(severityErrorRules, ","),
			strings.Join(severity.Severities(), ","))
	}

	return vsets, nil
}
