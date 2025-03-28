package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

const drugsURL = "https://rxnav.nlm.nih.gov/REST/rxclass/classMembers.json?classId=%s&relaSource=%s"

type RxClassResponse struct {
	DrugMemberGroup struct {
		DrugMembers []struct {
			RxCUI    string `json:"rxcui"`
			DrugName string `json:"drugName"`
		} `json:"drugMember"`
	} `json:"drugMemberGroup"`
}

type DrugsResponse struct {
	DrugMemberGroup struct {
		DrugMembers []struct {
			RxCUI    string `json:"rxcui"`
			DrugName string `json:"drugName"`
		} `json:"drugMember"`
	} `json:"drugMemberGroup"`
}

func getDrugsForClass(classID, relaSource string) ([]string, error) {
	url := fmt.Sprintf(drugsURL, classID, relaSource)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body) // Read response body for debugging
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var drugsResponse DrugsResponse
	err = json.Unmarshal(body, &drugsResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var drugNames []string
	for _, drugMember := range drugsResponse.DrugMemberGroup.DrugMembers {
		drugNames = append(drugNames, drugMember.DrugName)
	}

	return drugNames, nil
}

func main() {
	classIDs := []string{"N0000008638", "D009325"} // Example class IDs
	relaSources := []string{
		"ATC", "ATCPROD", "CDC", "DAILYMED", "FDASPL",
		"FMTSME", "MEDRT", "RXNORM", "SNOMEDCT", "VA",
	}

	for _, classID := range classIDs {
		for _, relaSource := range relaSources {
			drugNames, err := getDrugsForClass(classID, relaSource)
			if err != nil {
				log.Printf("Error retrieving drugs for class %s with relaSource %s: %v\n", classID, relaSource, err)
				continue
			}

			if len(drugNames) == 0 {
				log.Printf("No drugs found for class %s with relaSource %s\n", classID, relaSource)
				continue
			}

			fmt.Printf("Drugs for class %s (relaSource: %s):\n", classID, relaSource)
			for _, drugName := range drugNames {
				fmt.Println(drugName)
			}
		}
	}
}
