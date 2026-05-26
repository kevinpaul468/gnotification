package database

import (
	"reflect"
	"testing"
)

func TestDeepMergeMaps(t *testing.T) {
	tests := []struct {
		name string
		dst  map[string]interface{}
		src  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "empty dst",
			dst:  nil,
			src:  map[string]interface{}{"a": 1},
			want: map[string]interface{}{"a": 1},
		},
		{
			name: "empty src",
			dst:  map[string]interface{}{"a": 1},
			src:  nil,
			want: map[string]interface{}{"a": 1},
		},
		{
			name: "src overrides dst",
			dst:  map[string]interface{}{"host": "smtp.example.com", "port": 587, "from": "default@example.com"},
			src:  map[string]interface{}{"from": "app@example.com"},
			want: map[string]interface{}{"host": "smtp.example.com", "port": 587, "from": "app@example.com"},
		},
		{
			name: "src adds new keys",
			dst:  map[string]interface{}{"host": "smtp.example.com"},
			src:  map[string]interface{}{"port": 587, "from": "noreply@example.com"},
			want: map[string]interface{}{"host": "smtp.example.com", "port": 587, "from": "noreply@example.com"},
		},
		{
			name: "deep merge nested maps",
			dst:  map[string]interface{}{"tls": map[string]interface{}{"enabled": true}},
			src:  map[string]interface{}{"tls": map[string]interface{}{"cert": "/path/to/cert"}},
			want: map[string]interface{}{"tls": map[string]interface{}{"enabled": true, "cert": "/path/to/cert"}},
		},
		{
			name: "deep merge nested override",
			dst:  map[string]interface{}{"tls": map[string]interface{}{"enabled": true, "mode": "strict"}},
			src:  map[string]interface{}{"tls": map[string]interface{}{"mode": "relaxed"}},
			want: map[string]interface{}{"tls": map[string]interface{}{"enabled": true, "mode": "relaxed"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepMergeMaps(tt.dst, tt.src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("deepMergeMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConfigJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:  "valid JSON",
			input: `{"host":"smtp.example.com","port":587}`,
			want:  map[string]interface{}{"host": "smtp.example.com", "port": float64(587)},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfigJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConfigJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseConfigJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}
