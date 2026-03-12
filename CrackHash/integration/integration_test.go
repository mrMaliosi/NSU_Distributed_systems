package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"CrackHash/internal/api/http/dto"
)

const baseURL = "http://localhost:57107"

func startCrack(t *testing.T, req dto.CrackRequest) string {
	body, _ := json.Marshal(req)

	resp, err := http.Post(baseURL+"/api/hash/crack", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var r dto.CrackResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}

	if r.RequestID == "" {
		t.Fatal("requestId is empty")
	}

	return r.RequestID
}

func waitResult(t *testing.T, requestID string) []string {

	for i := 0; i < 60; i++ {

		resp, err := http.Get(baseURL + "/api/hash/status?requestId=" + requestID)
		if err != nil {
			t.Fatal(err)
		}

		var status dto.StatusResponse
		err = json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()

		if err != nil {
			t.Fatal(err)
		}

		switch status.Status {

		case "READY":
			return status.Data

		case "ERROR":
			t.Fatalf("error from API: %s", status.Error)

		case "CANCELLED":
			t.Fatal("task cancelled")

		case "IN_PROGRESS":
			time.Sleep(time.Second)

		default:
			t.Fatalf("unknown status %s", status.Status)
		}
	}

	t.Fatal("timeout waiting for result")
	return nil
}

func TestCrackSingleLetter(t *testing.T) {

	req := dto.CrackRequest{
		Hash:      "0cc175b9c0f1b6a831c399e269772661",
		MaxLength: 1,
		Algorithm: "MD5",
		Alphabet:  "abcdefghijklmnopqrstuvwxyz",
	}

	requestID := startCrack(t, req)
	data := waitResult(t, requestID)

	if len(data) != 1 || data[0] != "a" {
		t.Fatalf("expected 'a', got %v", data)
	}
}

func TestCrackSingleLetterSingleAlphabet(t *testing.T) {

	req := dto.CrackRequest{
		Hash:      "0cc175b9c0f1b6a831c399e269772661",
		MaxLength: 1,
		Algorithm: "MD5",
		Alphabet:  "a",
	}

	requestID := startCrack(t, req)
	data := waitResult(t, requestID)

	if len(data) != 1 || data[0] != "a" {
		t.Fatalf("expected 'a', got %v", data)
	}
}

func TestCrackLengthSeven(t *testing.T) {

	req := dto.CrackRequest{
		Hash:      "7ac66c0f148de9519b8bd264312c4d64", // пример MD5 для "abcdefg"
		MaxLength: 7,
		Algorithm: "MD5",
		Alphabet:  "abcdefg",
	}

	requestID := startCrack(t, req)
	data := waitResult(t, requestID)

	found := false
	for _, v := range data {
		if v == "abcdefg" {
			found = true
		}
	}

	if !found {
		t.Fatalf("expected 'abcdefg', got %v", data)
	}
}

func TestCrackLengthNine(t *testing.T) {

	req := dto.CrackRequest{
		Hash:      "8aa99b1f439ff71293e95357bac6fd94",
		MaxLength: 9,
		Algorithm: "MD5",
		Alphabet:  "abcdefghi",
	}

	requestID := startCrack(t, req)
	data := waitResult(t, requestID)

	found := false
	for _, v := range data {
		if v == "abcdefghi" {
			found = true
		}
	}

	if !found {
		t.Fatalf("expected 'abcdefghi', got %v", data)
	}
}
