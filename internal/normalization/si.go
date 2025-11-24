package normalization

import (
	"fmt"
	"regexp"
	"strings"
)

type SI struct {
	reStreet     *regexp.Regexp
	rePostalCode *regexp.Regexp
	reCity       *regexp.Regexp
	reStreetAbbr *regexp.Regexp
}

func NewSI() (*SI, error) {
	reStreet, err := regexp.Compile(`[+/(){}\[\]<>!§'$%&=?*#€¿_":;0-9-\n\r]`)
	if err != nil {
		return nil, err
	}

	rePostalCode, err := regexp.Compile(`[+/(){}\[\]<>!§'$%&=?*#€¿_":;\n\rA-Za-z]`)
	if err != nil {
		return nil, err
	}

	reCity, err := regexp.Compile(`[+/(){}\[\]<>!§'$%&=?*#€¿_",:;0-9\n\r]`)
	if err != nil {
		return nil, err
	}

	// Slovenske okrajšave ulic
	reStreetAbbr, err := regexp.Compile(`\b(ul\.|c\.|trg\.|nas\.|kol\.|pot\.|steza\.|cesta\.|ulica\.|trg\.|naselje\.|kolonija\.|pot\.|steza\.)\b`)
	if err != nil {
		return nil, err
	}

	return &SI{
		reStreet:     reStreet,
		rePostalCode: rePostalCode,
		reCity:       reCity,
		reStreetAbbr: reStreetAbbr,
	}, nil
}

func (si *SI) GetCountryCode() string {
	return "si"
}

func replaceSlovenianLetters(s string) string {
	s = strings.ReplaceAll(s, "č", "c")
	s = strings.ReplaceAll(s, "š", "s")
	s = strings.ReplaceAll(s, "ž", "z")
	s = strings.ReplaceAll(s, "Č", "c")
	s = strings.ReplaceAll(s, "Š", "s")
	s = strings.ReplaceAll(s, "Ž", "z")
	return s
}

func (si *SI) City(s string) (string, error) {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "/", " ")
	s = si.reCity.ReplaceAllString(s, "")
	s = replaceSlovenianLetters(s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimSpace(s) // Trim again after cleaning
	return s, nil
}

func (si *SI) PostalCode(s string) (string, error) {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = si.rePostalCode.ReplaceAllString(s, "")

	// Extract only digits
	var digits string
	for _, char := range s {
		if char >= '0' && char <= '9' {
			digits += string(char)
		}
	}

	if len(digits) < 4 {
		return digits, fmt.Errorf("postal code too short")
	}

	// If we have more than 4 digits, take the first 4
	if len(digits) > 4 {
		return digits[:4], nil
	}

	return digits, nil
}

func (si *SI) Street(s string) ([]string, error) {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ",", "|")
	s = strings.ReplaceAll(s, "\n", "|")

	// Slovenske okrajšave ulic
	s = strings.ReplaceAll(s, "ul.", "ulica")
	s = strings.ReplaceAll(s, "u. ", "ulica")
	s = strings.ReplaceAll(s, "c.", "cesta")
	s = strings.ReplaceAll(s, "trg.", "trg")
	s = strings.ReplaceAll(s, "nas.", "naselje")
	s = strings.ReplaceAll(s, "kol.", "kolonija")
	s = strings.ReplaceAll(s, "pot.", "pot")
	s = strings.ReplaceAll(s, "steza.", "steza")
	s = strings.ReplaceAll(s, "ul ", "ulica ")
	s = strings.ReplaceAll(s, "kol ", "kolonija ")
	s = strings.ReplaceAll(s, "nas ", "naselje ")

	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "/", " ")

	s = si.reStreet.ReplaceAllString(s, "")
	s = replaceSlovenianLetters(s)

	addressSlice := strings.Split(s, "|")
	var cleanAddressSlice []string
	for _, v := range addressSlice {
		var cleanedParts []string
		addressPartSlices := strings.Split(v, " ")
		for _, p := range addressPartSlices {
			if len(p) > 1 {
				cleanedParts = append(cleanedParts, p)
			}
		}
		cleanPart := strings.Join(cleanedParts, " ")
		if len(cleanPart) > 1 {
			cleanAddressSlice = append(cleanAddressSlice, cleanPart)
		}
	}
	return cleanAddressSlice, nil
}
