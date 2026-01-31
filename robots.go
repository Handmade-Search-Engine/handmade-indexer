package main

import (
	"math"
	"strconv"
	"strings"
)

type UserAgent struct {
	allow         []string
	disallow      []string
	crawlDelay    int
	contentSignal map[string]bool
}

func (userAgent *UserAgent) Copy() UserAgent {
	clone := UserAgent{
		crawlDelay: userAgent.crawlDelay,
	}

	clone.allow = make([]string, len(userAgent.allow))
	copy(clone.allow, userAgent.allow)

	clone.disallow = make([]string, len(userAgent.disallow))
	copy(clone.disallow, userAgent.disallow)

	clone.contentSignal = make(map[string]bool)
	for key, value := range userAgent.contentSignal {
		clone.contentSignal[key] = value
	}

	return clone
}

type Robots struct {
	agentRules map[string]UserAgent
	sitemap    string
}

func parseRobots(text string) Robots {
	lines := strings.Split(text, "\n")
	robots := Robots{}
	robots.agentRules = make(map[string]UserAgent)
	names := []string{}
	rules := UserAgent{}
	rules.contentSignal = make(map[string]bool)
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
				robots.agentRules[names[i]] = rules.Copy()
			}

			rules = UserAgent{}
			names = []string{}
			continue
		}
		if strings.Contains(lines[i], ":") == false {
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
			crawlDelay, err := strconv.ParseFloat(extractValue(lines[i]), 64)
			if err != nil {
				panic(err)
			}
			rules.crawlDelay = int(math.Ceil(crawlDelay))
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "content-signal") {
			signalString := extractValue(lines[i])
			signals := strings.Split(signalString, ",")
			for i := 0; i < len(signals); i++ {
				signal := strings.TrimSpace(signals[i])
				result := strings.SplitN(signal, "=", 2)
				signalName := result[0]

				signalValueString := result[1]
				signalValue := false
				if signalValueString == "yes" {
					signalValue = true
				}

				rules.contentSignal[signalName] = signalValue
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(lines[i]), "sitemap") {
			sitemap := extractValue(lines[i])
			robots.sitemap = sitemap
			continue
		}
		println(lines[i])
	}

	if len(names) > 0 {
		for _, name := range names {
			robots.agentRules[name] = rules.Copy()
		}
	}

	return robots
}

func extractValue(line string) string {
	return strings.TrimSpace(strings.SplitAfterN(line, ":", 2)[1])
}
