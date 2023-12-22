// Copyright 2023 David Sansome
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const accentsFile = "accents.txt"

var lineRegex = regexp.MustCompile(`^([^\t]+)\t([^\t]*)\t([^\t]+)$`)
var partsOfSpeechRegex = regexp.MustCompile(`\(([^)]+)\)`)
var commaRegex = regexp.MustCompile(`,`)
var semicolonRegex = regexp.MustCompile(`;`)

type possibleAccents struct {
	partsOfSpeech []string
	accents       []int
}

func (p possibleAccents) String() string {
	accentStrs := make([]string, len(p.accents))
	for i, accent := range p.accents {
		accentStrs[i] = strconv.Itoa(accent)
	}
	return fmt.Sprintf("%v;%v", strings.Join(p.partsOfSpeech, ","), strings.Join(accentStrs, ","))
}

type readingAndAccents struct {
	reading string
	accents []possibleAccents
}

func (r readingAndAccents) String() string {
	accentStrs := make([]string, len(r.accents))
	for i, accent := range r.accents {
		accentStrs[i] = accent.String()
	}
	return fmt.Sprintf("%s:%v", r.reading, strings.Join(accentStrs, "|"))
}

type index struct {
	vocabToReadingsAndAccents map[string][]readingAndAccents
}

func newIndex() *index {
	return &index{
		vocabToReadingsAndAccents: make(map[string][]readingAndAccents),
	}
}

func (idx *index) MarshalJSON() ([]byte, error) {
	toMarshal := make(map[string][]string)
	for vocab, readingsAndAccents := range idx.vocabToReadingsAndAccents {
		toMarshal[vocab] = make([]string, len(readingsAndAccents))
		for i, readingAndAccents := range readingsAndAccents {
			toMarshal[vocab][i] = readingAndAccents.String()
		}
	}
	return json.Marshal(toMarshal)
}

func (idx *index) add(vocab string, reading string, accents []possibleAccents) {
	idx.vocabToReadingsAndAccents[vocab] = append(idx.vocabToReadingsAndAccents[vocab], readingAndAccents{
		reading: reading,
		accents: accents,
	})
}

func parseLine(line []byte) (string, string, []possibleAccents, error) {
	matches := lineRegex.FindSubmatch(line)
	if matches == nil {
		return "", "", nil, errors.New(fmt.Sprintf("Failed to parse line: %s", line))
	}
	vocab := string(matches[1])
	// TODO: sometimes all or part of the reading will be in katakana. we may need to convert it to hiragana for it to
	// match what we're getting from wanikani
	reading := string(matches[2])
	if reading == "" {
		// for kana-only words, there is no separate reading listed
		reading = vocab
	}
	possibleAccents, err := parseAccentBytes(matches[3])
	if err != nil {
		return "", "", nil, err
	}
	return vocab, reading, possibleAccents, nil
}

func parseAccentBytes(accents []byte) ([]possibleAccents, error) {
	// accents is a comma-separated list of integers optionally prefixed by parts of speech in parens
	// e.g. "0,2",
	// e.g. "(名)2,(代)0,2"
	// e.g. "(名;代)2,(副)1,2"

	accentStrings := commaRegex.Split(string(accents), -1)

	partsOfSpeechToAccents := make(map[string][]int)
	currentPartOfSpeechKey := ""
	for _, accentString := range accentStrings {
		partsOfSpeechMatch := partsOfSpeechRegex.FindStringSubmatch(accentString)
		if partsOfSpeechMatch != nil {
			currentPartOfSpeechKey = partsOfSpeechMatch[1]
		}

		accentPart := partsOfSpeechRegex.ReplaceAllString(accentString, "")
		accent, err := strconv.Atoi(accentPart)
		if err != nil {
			return nil, err
		}
		partsOfSpeechToAccents[currentPartOfSpeechKey] = append(partsOfSpeechToAccents[currentPartOfSpeechKey], accent)
	}
	var res []possibleAccents
	for partOfSpeech, accents := range partsOfSpeechToAccents {
		res = append(res, possibleAccents{
			partsOfSpeech: semicolonRegex.Split(partOfSpeech, -1),
			accents:       accents,
		})
	}
	return res, nil
}

func main() {
	// open accentsFile to read line by line
	f, err := os.Open(accentsFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	idx := newIndex()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		vocab, reading, accents, err := parseLine(line)
		if err != nil {
			panic(err)
		}
		idx.add(vocab, reading, accents)
	}

	res, err := json.Marshal(idx)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(res))
}
