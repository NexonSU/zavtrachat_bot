package main

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Map local Bot API container paths onto the shared host volume.
func mediaLocalPaths(path string, botID int64) []string {
	var paths []string
	seen := make(map[string]bool)
	add := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	if filepath.IsAbs(path) || filepath.IsLocal(path) {
		add(path)
	} else {
		return nil
	}
	if Config == nil || Config.BotApiDataDir == "" {
		return paths
	}
	relative := mediaRelativePath(path, botID)
	if relative == "" {
		return paths
	}
	owners := []string{Config.Token, strconv.FormatInt(botID, 10)}
	if tokenID, _, ok := strings.Cut(Config.Token, ":"); ok {
		owners = append(owners, tokenID)
	}
	for _, owner := range owners {
		if owner != "" && owner != "0" {
			add(filepath.Join(Config.BotApiDataDir, owner, relative))
		}
	}
	add(filepath.Join(Config.BotApiDataDir, relative))
	return paths
}

func mediaRelativePath(path string, botID int64) string {
	relative := path
	token := ""
	if Config != nil {
		token = Config.Token
	}
	owners := []string{token, strconv.FormatInt(botID, 10)}
	if tokenID, _, ok := strings.Cut(token, ":"); ok {
		owners = append(owners, tokenID)
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	found := false
	for i, part := range parts {
		for _, owner := range owners {
			if owner != "" && owner != "0" && part == owner && i+1 < len(parts) {
				relative = filepath.FromSlash(strings.Join(parts[i+1:], "/"))
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if filepath.IsAbs(relative) || !filepath.IsLocal(relative) {
		return ""
	}
	return relative
}
