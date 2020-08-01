// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/dgrijalva/jwt-go"
)

// the functions in this file are copied, with light modification to fit this
// application, from Google's IAP example application:
// https://raw.githubusercontent.com/GoogleCloudPlatform/golang-samples/95634648f8e9140844db8af8244130491457e251/getting-started/authenticating-users/main.go

// iap holds the Cloud IAP certificates and audience field for this app, which
// are needed to verify authentication headers set by Cloud IAP.
type iap struct {
	certs map[string]string
	aud   string
}

// newIAP creates a new iap, returning an error if either the Cloud IAP
// certificates or audience field cannot be obtained.
func newIAP() (*iap, error) {
	certs, err := certificates()
	if err != nil {
		return nil, err
	}

	aud, err := audience()
	if err != nil {
		return nil, err
	}

	a := &iap{
		certs: certs,
		aud:   aud,
	}
	return a, nil
}

func (i *iap) Email(r *http.Request) string {
	assertion := r.Header.Get("X-Goog-IAP-JWT-Assertion")
	if assertion == "" {
		log.Fatal("No Cloud IAP header found.")
		return ""
	}

	email, _, err := validateAssertion(assertion, i.certs, i.aud)
	if err != nil {
		log.Fatalf("Could not validate assertion: %s", assertion)
		return ""
	}

	return email
}

// audience returns the expected audience value for this service.
func audience() (string, error) {
	projectNumber, err := metadata.NumericProjectID()
	if err != nil {
		return "", fmt.Errorf("metadata.NumericProjectID: %v", err)
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		return "", fmt.Errorf("metadata.ProjectID: %v", err)
	}

	return "/projects/" + projectNumber + "/apps/" + projectID, nil
}

// certificates returns Cloud IAP's cryptographic public keys.
func certificates() (map[string]string, error) {
	const url = "https://www.gstatic.com/iap/verify/public_key"
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Get: %v", err)
	}

	var certs map[string]string
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&certs); err != nil {
		return nil, fmt.Errorf("Decode: %v", err)
	}

	return certs, nil
}

// validateAssertion validates assertion was signed by Google and returns the
// associated email and userID.
func validateAssertion(assertion string, certs map[string]string, aud string) (email string, userID string, err error) {
	token, err := jwt.Parse(assertion, func(token *jwt.Token) (interface{}, error) {
		keyID := token.Header["kid"].(string)

		_, ok := token.Method.(*jwt.SigningMethodECDSA)
		if !ok {
			return nil, fmt.Errorf("unexpected signing method: %q", token.Header["alg"])
		}

		cert := certs[keyID]
		return jwt.ParseECPublicKeyFromPEM([]byte(cert))
	})

	if err != nil {
		return "", "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", fmt.Errorf("could not extract claims (%T): %+v", token.Claims, token.Claims)
	}

	if claims["aud"].(string) != aud {
		return "", "", fmt.Errorf("mismatched audience. aud field %q does not match %q", claims["aud"], aud)
	}
	return claims["email"].(string), claims["sub"].(string), nil
}
