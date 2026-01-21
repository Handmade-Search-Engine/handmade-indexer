package main

import (
	"strconv"
	"strings"
)

func main() {
	example := `
User-agent: *
Content-Signal: ai-train=no, search=yes, ai-input=no
Allow: /
`
	parse(example)
}

type ContentSignal struct {
	aiInput bool
	aiTrain bool
	search  bool
}
type UserAgent struct {
	allow         []string
	disallow      []string
	crawlDelay    int
	contentSignal ContentSignal
}
type Robots struct {
	agentRules map[string]UserAgent
	sitemap    string
}

func parse(text string) Robots {
	lines := strings.Split(text, "\n")
	robots := Robots{}
	robots.agentRules = make(map[string]UserAgent)
	names := []string{}
	rules := UserAgent{}
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "#") {
			continue
		}
		if len(lines[i]) == 0 {
			if len(names) == 0 {
				// last line was either empty or a comment
				continue
			}

			for i := 0; i < len(names); i++ {
				rulesCopy := rules
				robots.agentRules[names[i]] = rulesCopy
			}

			rules = UserAgent{}
			names = []string{}
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "user-agent") {
			name := extractValue(lines[i])
			names = append(names, name)
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "disallow") {
			location := extractValue(lines[i])
			rules.disallow = append(rules.disallow, location)
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "allow") {
			location := extractValue(lines[i])
			rules.allow = append(rules.allow, location)
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "crawl-delay") {
			crawlDelay, err := strconv.Atoi(extractValue(lines[i]))
			if err != nil {
				panic(err)
			}
			rules.crawlDelay = crawlDelay
			continue
		}
		println(lines[i])
	}
	return Robots{}
}

func extractValue(line string) string {
	return strings.TrimSpace(strings.SplitAfterN(line, ":", 2)[1])
}
