package swagger2

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/ghodss/yaml"
)

func ReadOpenAPI2SpecFile(filename string) (*Specification, error) {
	spec, err := ReadOpenAPI2SpecFileDirect(filename)
	return &spec, err
}

func ReadSwagger2SpecFile(filepath string) (Specification, error) {
	return ReadOpenAPI2SpecFileDirect(filepath)
}

func ReadOpenAPI2SpecFileDirect(filename string) (Specification, error) {
	var spec Specification
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return spec, err
	}
	rx := regexp.MustCompile(`.ya?ml$`)
	if rx.MatchString(strings.ToLower(strings.TrimSpace(filename))) {
		err = yaml.Unmarshal(bytes, &spec)
	} else {
		err = json.Unmarshal(bytes, &spec)
	}
	return spec, err
}

/*func ReadSwagger2Spec(filepath string) (Specification, error) {
	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return Specification{}, err
	}
	return NewSpecificationFromBytes(bytes)
}*/

func ReadOpenAPI2KinSpecFile(filename string) (*openapi2.Swagger, error) {
	var swag openapi2.Swagger
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return &swag, err
	}
	rx := regexp.MustCompile(`.ya?ml$`)
	if rx.MatchString(strings.ToLower(strings.TrimSpace(filename))) {
		err = yaml.Unmarshal(bytes, &swag)
	} else {
		err = json.Unmarshal(bytes, &swag)
	}
	return &swag, err
}
