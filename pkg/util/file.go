package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadFileAndFixNewline reads the content of a io.Reader and replaces \r\n with \n
func ReadFileAndFixNewline(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return []byte(strings.ReplaceAll(string(content), "\r\n", "\n")), nil
}

func appendAndNL(to, from *[]byte) {
	if from != nil {
		*to = append(*to, *from...)
	}
	*to = append(*to, '\n')
}

func appendAndNLStr(to *[]byte, from string) {
	*to = append(*to, from...)
	*to = append(*to, '\n')
}

// PrefixFirstYamlDocument inserts a line to the beginning of the first YAML document in a file having content
func PrefixFirstYamlDocument(line, file string) error {
	fileInfo, err := os.Stat(file)
	if err != nil {
		return err
	}
	perm := fileInfo.Mode().Perm()
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	eol := "\n"
	if len(content) >= 2 && content[len(content)-2] == '\r' && content[len(content)-1] == '\n' {
		eol = "\r\n"
	}

	// put line directly below YAML document_start if it exists and nothing is preceding it
	documentStart := "---" + eol
	if strings.HasPrefix(string(content), documentStart) {
		content = content[len(documentStart):]
		line = documentStart + line
	}

	newContent := line + eol + string(content)
	return os.WriteFile(file, []byte(newContent), perm)
}

// RemoveCommentsFromYaml tries to remove comments if they contain valid yaml
func RemoveCommentsFromYaml(reader io.Reader) ([]byte, error) {
	result := make([]byte, 0)
	scanner := bufio.NewScanner(reader)

	helmDocsMatcher := regexp.MustCompile(`^\s*#\s*--`)
	commentMatcher := regexp.MustCompile(`^(\s*#\s*)(.*$)`)
	commentYamlMapMatcher := regexp.MustCompile(`^(\s*#\s*)([^:]+:)(.*$)`)
	whitespaceMatcher := regexp.MustCompile(`\s`)
	schemaMatcher := regexp.MustCompile(`^\s*#\s@schema\s*`)

	var line string
	var inDocs, inSchema bool
	var unknownYaml interface{}
	var headerCommentsParsed bool

	for scanner.Scan() {
		line = scanner.Text()

		// Skip uncommenting the first comment block in the file, e.g. for when using something like # yaml-language-server: $schema=<urlToTheSchema>
		if !headerCommentsParsed {
			if commentMatcher.Match([]byte(line)) && !schemaMatcher.Match([]byte(line)) && !helmDocsMatcher.Match([]byte(line)) {
				appendAndNLStr(&result, line)
				continue
			} else {
				headerCommentsParsed = true
			}

		}

		// Don't try to uncomment helm-docs descriptions
		if helmDocsMatcher.Match([]byte(line)) {
			inDocs = true
			appendAndNLStr(&result, line)
			continue
		}

		// Line contains @schema
		// The following lines will be added to result
		if schemaMatcher.Match([]byte(line)) {
			inSchema = !inSchema
			appendAndNLStr(&result, line)
			continue
		}

		// Inside a @schema
		if inSchema {

			appendAndNLStr(&result, line)
			continue
		}

		matches := commentYamlMapMatcher.FindStringSubmatch(line)
		// Inside helm-docs block
		if inDocs {
			if len(matches) == 0 {
				appendAndNLStr(&result, line)
				continue
			} else {
				// check for the potential case of having <text>: inside helm-docs comment, this will only fail if the comment line actually starts with <text>:
				if whitespaceMatcher.MatchString(matches[2]) {
					appendAndNLStr(&result, line)
					continue
				}
				inDocs = false
			}

		}

		// If this matches a potential commented yaml
		matches = commentMatcher.FindStringSubmatch(line)
		if len(matches) > 0 {
			// Strip the comment away

			cleanWhitespace := strings.Replace(matches[1], "#", "", 1)
			whitespaceSize := len(cleanWhitespace)
			// Check if the number of spaces is even, if it's not it's likely someone added an extra whitespace along with the #
			if whitespaceSize > 0 && whitespaceSize&1 == 1 {
				cleanWhitespace = cleanWhitespace[1:]
			}
			// join the whitespace with the data
			strippedLine := cleanWhitespace + matches[2]

			// add it to the already parsed valid yaml
			appendAndNLStr(&result, strippedLine)

			// If the line is not a comment it must be yaml
			//appendAndNLStr(&buff, line)
			//continue
		} else {
			// line is not a commented yaml
			appendAndNLStr(&result, line)
		}
	}
	// check if the new block is still valid yaml
	err := yaml.Unmarshal(result, &unknownYaml)
	if err != nil {
		// Invalid yaml found,
		fmt.Println(err)
		panic("Invalid yaml after uncommenting:\n" + string(result))
	}

	return result, nil
}

// IsRelativeFile checks if the given string is a relative path to a file
func IsRelativeFile(root, relPath string) (string, error) {
	if !path.IsAbs(relPath) {
		foo := path.Join(path.Dir(root), relPath)
		_, err := os.Stat(foo)
		return foo, err
	}
	return "", errors.New("Is absolute file")
}
