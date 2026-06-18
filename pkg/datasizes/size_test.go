package datasizes_test

import (
	"encoding/json"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/pkg/datasizes"
)

func TestSizeUnmarshalTOMLUnhappy(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "wrong datatype/bool",
			input: `size = true`,
			err:   `toml: line 1 (last key "size"): error decoding TOML size: failed to convert value "true" to number`,
		},
		{
			name:  "wrong datatype/float",
			input: `size = 3.14`,
			err:   `toml: line 1 (last key "size"): error decoding TOML size: cannot be float`,
		},
		{
			name:  "wrong unit",
			input: `size = "20 KG"`,
			err:   `toml: line 1 (last key "size"): error decoding TOML size: unknown data size units in string: 20 KG`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v struct {
				Size datasizes.Size `toml:"size"`
			}
			err := toml.Unmarshal([]byte(tc.input), &v)
			assert.EqualError(t, err, tc.err, tc.input)
		})
	}
}

func TestSizeUnmarshalJSONUnhappy(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "misize nor string nor int",
			input: `{"size": true}`,
			err:   `error decoding size: failed to convert value "true" to number`,
		},
		{
			name:  "wrong datatype/float",
			input: `{"size": 3.14}`,
			err:   `error decoding size: strconv.ParseInt: parsing "3.14": invalid syntax`,
		},
		{
			name:  "misize not parseable",
			input: `{"size": "20 KG"}`,
			err:   `error decoding size: unknown data size units in string: 20 KG`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v struct {
				Size datasizes.Size `json:"size"`
			}
			err := json.Unmarshal([]byte(tc.input), &v)
			assert.EqualError(t, err, tc.err, tc.input)
		})
	}
}

func TestSizeUnmarshalYAMLUnhappy(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "misize nor string nor int",
			input: `size: true`,
			err:   `unmarshal yaml via json for true failed: error decoding size: failed to convert value "true" to number`,
		},
		{
			name:  "wrong datatype/float",
			input: `size: 3.14`,
			err:   `unmarshal yaml via json for 3.14 failed: error decoding size: strconv.ParseInt: parsing "3.14": invalid syntax`,
		},
		{
			name:  "misize not parseable",
			input: `size: "20 KG"`,
			err:   `unmarshal yaml via json for "20 KG" failed: error decoding size: unknown data size units in string: 20 KG`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v struct {
				Size datasizes.Size `json:"size"`
			}
			err := yaml.Unmarshal([]byte(tc.input), &v)
			assert.EqualError(t, err, tc.err, tc.input)
		})
	}
}

func TestSizeUnmarshalHappy(t *testing.T) {
	cases := []struct {
		name      string
		inputJSON string
		inputTOML string
		inputYAML string
		expected  datasizes.Size
	}{
		{
			name:      "int",
			inputJSON: `{"size": 1234}`,
			inputTOML: `size = 1234`,
			inputYAML: `size: 1234`,
			expected:  1234,
		},
		{
			name:      "str",
			inputJSON: `{"size": "1234"}`,
			inputTOML: `size = "1234"`,
			inputYAML: `size: "1234"`,
			expected:  1234,
		},
		{
			name:      "str/with-unit",
			inputJSON: `{"size": "1234 MiB"}`,
			inputTOML: `size = "1234 MiB"`,
			inputYAML: `size: "1234 MiB"`,
			expected:  1234 * datasizes.MiB,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v struct {
				Size datasizes.Size `json:"size" toml:"size"`
			}
			err := toml.Unmarshal([]byte(tc.inputTOML), &v)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, v.Size, tc.inputTOML)
			err = json.Unmarshal([]byte(tc.inputJSON), &v)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, v.Size, tc.inputJSON)

			err = yaml.Unmarshal([]byte(tc.inputYAML), &v)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, v.Size, tc.inputYAML)
		})
	}
}

func TestSizeUint64(t *testing.T) {
	assert.Equal(t, datasizes.Size(1234).Uint64(), uint64(1234))
}
