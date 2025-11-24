package si

import (
	"openPAQ/internal/algorithms"
	"openPAQ/internal/normalization"
	"openPAQ/internal/types"
	"slices"

	"github.com/sirupsen/logrus"
)

type NormalizeEnclosure struct {
	Raw        string
	Normalized string
}

type CityPostalCodeItems struct {
	City        string
	PostalCodes []NormalizeEnclosure
}

type PostalCodeStreetItems struct {
	PostalCode string
	Streets    []NormalizeEnclosure
}

type SI struct {
	db            Database
	matcherConfig algorithms.MatchSeverityConfig

	postalCodeStreet  map[string]PostalCodeStreetItems
	cityPostalCode    map[string]CityPostalCodeItems
	firstLetterCities map[string][]string //key first letter of city

	normalize normalization.Normalize
}

func NewSI(database Database, normalizer normalization.Normalize, matcherConfig algorithms.MatchSeverityConfig) *SI {
	si := &SI{
		normalize:     normalizer,
		matcherConfig: matcherConfig,
		db:            database,
	}

	si.buildMatchingLists()

	return si
}

func (si *SI) buildMatchingLists() {

	logrus.Info("Building PostalCodeStreet for SI")
	postalCodeStreet, err := si.db.GetPostalCodeStreet(si.normalize)

	if err != nil {
		panic(err)
	}

	si.postalCodeStreet = postalCodeStreet

	logrus.Info("Building CityPostalCode for SI")

	cityPostalCode, err := si.db.GetCityPostalCode(si.normalize)
	if err != nil {
		panic(err)
	}

	si.cityPostalCode = cityPostalCode

	logrus.Info("Building FirstLetterCities for SI")

	si.buildFirstLetterCities()

	logrus.Info("Building indexes for SI finished")

}

func (si *SI) buildFirstLetterCities() {

	var cities []string

	for city := range si.cityPostalCode {
		cities = append(cities, city)
	}

	slices.Sort(cities)
	cities = slices.Compact(cities)

	si.firstLetterCities = make(map[string][]string)

	for _, city := range cities {

		cityFirstLetter := string([]rune(city)[0])

		if _, ok := si.firstLetterCities[cityFirstLetter]; !ok {
			si.firstLetterCities[cityFirstLetter] = []string{city}
			continue
		}

		si.firstLetterCities[cityFirstLetter] = append(si.firstLetterCities[cityFirstLetter], city)

	}
}

func (si *SI) GetCountryCode() string {
	return "si"
}

func (si *SI) CityStreetCheck(input types.NormalizeInput) chan types.PairMatching {

	reChan := make(chan types.PairMatching, 1)

	go func(c chan types.PairMatching) {
		defer close(c)
		result := types.PairMatching{}

		cityConfig := si.matcherConfig
		cityConfig.AlgorithmThreshold = 0.9
		cityConfig.AllowPartialMatch = true
		cityConfig.AllowPartialCompareListMatch = true
		cityConfig.PartialInputSeparators = []string{" ", "/", "-"}

		if input.City == "" {
			c <- result
			return
		}

		inputCityFirstLetter := string([]rune(input.City)[0])

		cityCandidates, err := algorithms.GetMatches(input.City, si.firstLetterCities[inputCityFirstLetter], cityConfig)
		if err != nil {
			c <- result
			return
		}

		var matches []types.CityStreetPostalCode
		const maxMatches = 50              // Limit total matches to prevent huge result sets
		const perfectMatchThreshold = 0.95 // Early exit if we find a near-perfect match

		for _, cityCandidate := range cityCandidates {
			if len(matches) >= maxMatches {
				break // Early exit if we have enough matches
			}

			cityPostalCodeItem, ok := si.cityPostalCode[cityCandidate.Value]

			if ok {
				// Limit postal codes checked to prevent checking too many
				postalCodesToCheck := cityPostalCodeItem.PostalCodes
				if len(postalCodesToCheck) > 10 {
					postalCodesToCheck = postalCodesToCheck[:10] // Only check first 10 postal codes
				}

				for _, postalCode := range postalCodesToCheck {
					if len(matches) >= maxMatches {
						break
					}

					postalCodeStreetItems, foundPostalCode := si.postalCodeStreet[postalCode.Normalized]

					if foundPostalCode {
						res := si.PostalCodeStreetCheck(types.NormalizeInput{
							Streets:    input.Streets,
							PostalCode: postalCode.Normalized,
						})

						a := <-res

						if a.PostalCodeStreetMatch {
							// Limit street matches per postal code
							streetMatchesToAdd := a.PostalCodeStreetMatches
							if len(streetMatchesToAdd) > 20 {
								streetMatchesToAdd = streetMatchesToAdd[:20] // Only top 20 matches
							}

							hasPerfectMatch := false
							for _, street := range streetMatchesToAdd {
								if len(matches) >= maxMatches {
									break
								}

								// Check for perfect match for early exit
								if street.StreetSimilarity >= perfectMatchThreshold && cityCandidate.Similarity >= perfectMatchThreshold {
									hasPerfectMatch = true
								}

								matches = append(matches, types.CityStreetPostalCode{
									City:                  cityPostalCodeItem.City,
									Street:                street.Street,
									PostalCode:            postalCodeStreetItems.PostalCode,
									CountryCode:           "si",
									StreetSimilarity:      street.StreetSimilarity,
									WasPartialStreetMatch: street.WasPartialStreetMatch,
									CitySimilarity:        cityCandidate.Similarity,
									WasPartialCityMatch:   cityCandidate.WasPartial,
									WasListMatch:          true,
								})
							}

							// Early exit if we found a perfect match
							if hasPerfectMatch && len(matches) > 0 {
								break
							}
						}
					}
				}
			}
		}

		if len(matches) > 0 {
			result = types.PairMatching{
				StreetCityMatch:   true,
				StreetCityMatches: matches,
			}
		}

		c <- result
	}(reChan)

	return reChan
}

func (si *SI) PostalCodeStreetCheck(input types.NormalizeInput) chan types.PairMatching {

	reChan := make(chan types.PairMatching, 1)

	go func(c chan types.PairMatching) {
		defer close(c)
		result := types.PairMatching{}

		postalCodeItem, ok := si.postalCodeStreet[input.PostalCode]

		var matches []types.PostalCodeStreet
		const maxMatchesPerStreet = 10     // Limit matches per input street
		const perfectMatchThreshold = 0.95 // Early exit for perfect matches

		if ok {
			var normalizedStreets []string

			// Limit the number of streets we check for large postal codes to improve performance
			maxStreetsToCheck := 5000 // Reasonable limit for large postal codes
			streetsToCheck := postalCodeItem.Streets
			if len(streetsToCheck) > maxStreetsToCheck {
				streetsToCheck = streetsToCheck[:maxStreetsToCheck]
			}

			for _, streetFromList := range streetsToCheck {
				normalizedStreets = append(normalizedStreets, streetFromList.Normalized)
			}
			for _, inputStreetItem := range input.Streets {
				var streetCandidates []algorithms.MatchResult
				var err error

				// Stage 1: Fastest - no partial matching, just exact/fuzzy matching
				streetConfig := si.matcherConfig
				streetConfig.AllowPartialMatch = false
				streetConfig.AllowPartialCompareListMatch = false
				streetConfig.AllowCombineAllForwardCombinations = false

				streetCandidates, err = algorithms.GetMatches(inputStreetItem, normalizedStreets, streetConfig)

				// Stage 2: Enable partial matching (but not compare list) - still fast
				if err != nil || len(streetCandidates) == 0 {
					streetConfig.AllowPartialMatch = true
					streetConfig.AllowPartialCompareListMatch = false
					streetConfig.AllowCombineAllForwardCombinations = false
					streetConfig.PartialInputSeparators = []string{" "}
					streetConfig.PartialCompareListSeparators = []string{" "}

					streetCandidates, err = algorithms.GetMatches(inputStreetItem, normalizedStreets, streetConfig)
				}

				// Stage 3: Enable partial compare list matching - more expensive
				if err != nil || len(streetCandidates) == 0 {
					streetConfig.AllowPartialMatch = true
					streetConfig.AllowPartialCompareListMatch = true
					streetConfig.AllowCombineAllForwardCombinations = false
					streetConfig.PartialInputSeparators = []string{" ", "/", "-"}
					streetConfig.PartialCompareListSeparators = []string{" ", "/", "-"}

					streetCandidates, err = algorithms.GetMatches(inputStreetItem, normalizedStreets, streetConfig)
				}

				// Stage 4: Enable all forward combinations - most expensive, last resort
				if err != nil || len(streetCandidates) == 0 {
					streetConfig.AllowPartialMatch = true
					streetConfig.AllowPartialCompareListMatch = true
					streetConfig.AllowCombineAllForwardCombinations = true
					streetConfig.PartialInputSeparators = []string{" ", "/", "-"}
					streetConfig.PartialCompareListSeparators = []string{" ", "/", "-"}

					streetCandidates, err = algorithms.GetMatches(inputStreetItem, normalizedStreets, streetConfig)
				}

				if err != nil {
					continue
				}

				// Limit candidates and check for perfect match for early exit
				candidatesToProcess := streetCandidates
				if len(candidatesToProcess) > maxMatchesPerStreet {
					candidatesToProcess = candidatesToProcess[:maxMatchesPerStreet]
				}

				foundPerfectMatch := false
				for _, streetCandidate := range candidatesToProcess {
					// Early exit if we found a perfect match
					if streetCandidate.Similarity >= perfectMatchThreshold {
						foundPerfectMatch = true
					}

					for _, street := range streetsToCheck {
						if street.Normalized == streetCandidate.Value {
							matches = append(matches, types.PostalCodeStreet{
								PostalCode:            postalCodeItem.PostalCode,
								Street:                street.Raw,
								CountryCode:           "si",
								StreetSimilarity:      streetCandidate.Similarity,
								WasPartialStreetMatch: streetCandidate.WasPartial,
								WasListMatch:          true,
							})
							break // Only add first match for this candidate
						}
					}

					// Early exit if we found a perfect match and have at least one result
					if foundPerfectMatch && len(matches) > 0 {
						break
					}
				}
			}
		}

		if len(matches) > 0 {
			result = types.PairMatching{
				PostalCodeStreetMatch:   true,
				PostalCodeStreetMatches: matches,
			}
		}

		c <- result
	}(reChan)

	return reChan
}

func (si *SI) PostalCodeCityCheck(input types.NormalizeInput) chan types.PairMatching {
	reChan := make(chan types.PairMatching, 1)

	go func(c chan types.PairMatching) {
		defer close(c)
		result := types.PairMatching{}

		cityConfig := si.matcherConfig
		cityConfig.AlgorithmThreshold = 0.9
		cityConfig.AllowPartialMatch = true
		cityConfig.AllowPartialCompareListMatch = true
		cityConfig.PartialInputSeparators = []string{" ", "/", "-"}

		if input.City == "" {
			c <- result
			return
		}

		inputCityFirstLetter := string([]rune(input.City)[0])
		cityCandidates, err := algorithms.GetMatches(input.City, si.firstLetterCities[inputCityFirstLetter], cityConfig)
		if err != nil {
			c <- result
			return
		}

		var matches []types.CityPostalCode

		for _, cityCandidate := range cityCandidates {
			cityPostalCodeItem := si.cityPostalCode[cityCandidate.Value]
			for _, postalCodeCandidate := range cityPostalCodeItem.PostalCodes {

				if postalCodeCandidate.Normalized == input.PostalCode {
					matches = append(matches, types.CityPostalCode{
						City:                cityPostalCodeItem.City,
						PostalCode:          postalCodeCandidate.Raw,
						CountryCode:         "si",
						CitySimilarity:      cityCandidate.Similarity,
						WasPartialCityMatch: cityCandidate.WasPartial,
						WasListMatch:        true,
					})
				}
			}
		}

		if len(matches) > 0 {
			result = types.PairMatching{
				CityPostalCodeMatch:   true,
				CityPostalCodeMatches: matches,
			}
		}

		c <- result

	}(reChan)

	return reChan
}
