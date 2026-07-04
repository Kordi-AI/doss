// Package policy decides what may be told to whom. It is deliberately
// boring: deterministic matching, first rule wins, no rule means nothing.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Rule struct {
	About string `yaml:"about"`
	To    string `yaml:"to"`
	Give  string `yaml:"give"`
}

type Policy struct {
	Groups map[string][]string `yaml:"groups"`
	Rules  []Rule              `yaml:"rules"`
}

// Load reads policy.yaml. Any problem fails closed: the caller must treat
// an error as "share nothing".
func Load(vaultDir string) (*Policy, error) {
	b, err := os.ReadFile(filepath.Join(vaultDir, "policy.yaml"))
	if err != nil {
		return nil, fmt.Errorf("policy.yaml unreadable — failing closed, nothing will be shared: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("policy.yaml invalid — failing closed, nothing will be shared: %v", err)
	}
	return &p, nil
}

// Give returns the disclosure level for requester × topic.
// First matching rule wins (like a firewall); no match means "nothing".
func (p *Policy) Give(requester, topic string) string {
	for _, r := range p.Rules {
		if p.audienceMatches(r.To, requester) && topicMatches(r.About, topic) {
			if r.Give == "" {
				return "nothing"
			}
			return r.Give
		}
	}
	return "nothing"
}

func (p *Policy) audienceMatches(to, requester string) bool {
	if to == "anyone" || to == "*" || to == requester {
		return true
	}
	for _, member := range p.Groups[to] {
		if member == requester {
			return true
		}
	}
	return false
}

func topicMatches(pattern, topic string) bool {
	if pattern == "*" || pattern == topic {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		return strings.HasPrefix(topic, strings.TrimSuffix(pattern, "*"))
	}
	return false
}
