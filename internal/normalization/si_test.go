package normalization

import (
	"reflect"
	"testing"
)

func TestSIPostalCode(t *testing.T) {
	si, err := NewSI()
	if err != nil {
		t.Fatalf("failed to init SI normalizer: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "clean postal code", input: "1000", want: "1000"},
		{name: "with prefix and spaces", input: "SI-1 2 3 4", want: "1234"},
		{name: "letters sprinkled in", input: "S1B0L3A5", want: "1035"},
		{name: "too short remains error", input: "12", want: "12", wantErr: true},
		{name: "truncate excessive digits", input: "1234567", want: "1234"},
		{name: "remove non digits keep first four", input: "A9B8C7D6", want: "9876"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := si.PostalCode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PostalCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("PostalCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSICity(t *testing.T) {
	si, err := NewSI()
	if err != nil {
		t.Fatalf("failed to init SI normalizer: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercase city", input: "Ljubljana", want: "ljubljana"},
		{name: "remove punctuation and digits", input: "Maribor!!! 2000", want: "maribor"},
		{name: "trim spaces", input: "  Celje  ", want: "celje"},
		{name: "slovenian characters replaced", input: "Škofja Loka", want: "skofja loka"},
		{name: "newline and hyphen handling", input: "\nNova-Gorica\n", want: "nova gorica"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := si.City(tt.input)
			if err != nil {
				t.Fatalf("City() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("City() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSIStreet(t *testing.T) {
	si, err := NewSI()
	if err != nil {
		t.Fatalf("failed to init SI normalizer: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple street without noise",
			input: "Trg republike 3",
			want:  []string{"trg republike"},
		},
		{
			name:  "handles abbreviations and mixed case",
			input: "UL. heroja Staneta 12, C. na vrh 8",
			want:  []string{"ulica heroja staneta", "cesta na vrh"},
		},
		{
			name:  "multiline and crazy punctuation",
			input: "Nas. narodnih herojev 3\nPot. na poljane 17!?!",
			want:  []string{"naselje narodnih herojev", "pot na poljane"},
		},
		{
			name:  "human errors extra separators and typos",
			input: "  Trg. revolucije,,,   14 ||| Kol. mladinskih delavcev 5b ",
			want:  []string{"trg revolucije", "kolonija mladinskih delavcev"},
		},
		{
			name:  "multiple spaces remove short tokens",
			input: "Cesta 27. aprila 002\nSteza.. bratov 19",
			want:  []string{"cesta aprila", "steza bratov"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := si.Street(tt.input)
			if err != nil {
				t.Fatalf("Street() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Street() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

