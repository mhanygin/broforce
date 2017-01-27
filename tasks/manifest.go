package tasks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Jeffail/gabs"
	"github.com/google/go-github/github"
	"github.com/xanzy/go-gitlab"

	"github.com/InnovaCo/broforce/bus"
	"github.com/InnovaCo/broforce/config"
	"github.com/InnovaCo/broforce/logger"
)

const (
	defaultSHA   = "0000000000000000000000000000000000000000"
	manifestName = "manifest.yml"
)

func init() {
	registry("manifest", bus.Task(&manifest{}))
}

type manifest struct {
	config config.ConfigData
	bus    *bus.EventsBus
}

type serveParams struct {
	Vars     map[string]string `json:"vars"`
	Manifest []byte            `json:"manifest"`
	Plugin   string            `json:"plugin"`
	Ref      string
}

func (p *manifest) Run(eventBus *bus.EventsBus, cfg config.ConfigData) error {
	p.bus = eventBus
	p.config = cfg
	p.bus.Subscribe(bus.GitlabHookEvent, p.handlerGitlab)
	p.bus.Subscribe(bus.GithubHookEvent, p.handlerGithub)

	return nil
}

func (p *manifest) handlerGitlab(e bus.Event) error {
	var host, token = p.config.GetString("gitlab.host"), p.config.GetString("gitlab.token")
	params := serveParams{Vars: map[string]string{"purge": "false"}}

	logger.Log.Infof("%v %v", token, host)

	g, err := gabs.ParseJSON(e.Data)
	if err != nil {
		return err
	}
	projectId, ok := g.Search("project_id").Data().(float64)
	if !ok {
		return fmt.Errorf("Key %s not found", "project_id")
	}
	params.Vars["ssh-repo"], ok = g.Search("repository", "url").Data().(string)
	if !ok {
		return fmt.Errorf("Key %s not found", "repository.url")
	}

	if ref, ok := g.Search("ref").Data().(string); !ok {
		return fmt.Errorf("Key %s not found", "ref")
	} else {
		s := strings.Split(ref, "/")
		params.Vars["branch"] = s[len(s)-1]
		params.Ref = params.Vars["branch"]
	}

	logger.Log.Debugf("%v %v", token, projectId)

	if after, ok := g.Search("after").Data().(string); ok && (strings.Compare(after, defaultSHA) == 0) {
		params.Vars["purge"] = "true"
		if params.Ref, ok = g.Search("before").Data().(string); !ok {
			return fmt.Errorf("Key %s not found", "before")
		}
	} else {
		m := false
		commits, _ := g.S("commits").Children()
		for _, commit := range commits {
			modified, _ := commit.S("modified").Children()
			added, _ := commit.S("added").Children()
			for _, f := range append(modified, added...) {
				if strings.Compare(f.Data().(string), manifestName) != -1 {
					m = true
					break
				}
			}
		}
		if !m {
			if before, ok := g.Search("before").Data().(string); ok && strings.Compare(before, defaultSHA) != 0 {
				return fmt.Errorf("%s not change", manifestName)
			}
		}
	}

	if params.Manifest, err = p.uploadGitlabManifest(host, token, fmt.Sprintf("%v", projectId), params.Ref, manifestName); err != nil {
		return err
	}

	p.pusher(e.Trace, []string{"gocd.pipeline.create", "db.create"}, params)
	if strings.Compare(params.Vars["purge"], "true") == 0 {
		p.pusher(e.Trace, []string{"outdated"}, params)
	}
	return nil
}

func (p *manifest) pusher(uuid string, plugins []string, params serveParams) {
	e := bus.Event{
		Trace:   uuid,
		Subject: bus.ServeCmdEvent,
		Coding:  bus.JsonCoding}

	for _, plugin := range plugins {
		params.Plugin = plugin
		if err := bus.Coder(&e, params); err != nil {
			logger.Log.Error(err)
			continue
		}
		if err := p.bus.Publish(e); err != nil {
			logger.Log.Error(err)
		}
	}
}

func (p *manifest) handlerGithub(e bus.Event) error {
	g, err := gabs.ParseJSON(e.Data)
	if err != nil {
		return err
	}

	var host, token = p.config.GetString("github.host"), p.config.GetString("github.token")
	params := serveParams{Vars: map[string]string{"purge": "false"}}

	logger.Log.Infof("%v %v", token, host)

	repo, ok := g.Search("repository", "contents_url").Data().(string)
	if !ok {
		return fmt.Errorf("Key %s not found", "repository.contents_url")
	}
	repo = strings.Replace(repo, "{+path}", "", 1)

	params.Vars["ssh-repo"], ok = g.Search("repository", "url").Data().(string)
	if !ok {
		return fmt.Errorf("Key %s not found", "repository.url")
	}

	if b, ok := g.Search("ref").Data().(string); !ok {
		return fmt.Errorf("Key %s not found", "ref")
	} else {
		s := strings.Split(b, "/")
		params.Vars["branch"] = s[len(s)-1]
		params.Ref = params.Vars["branch"]
	}

	logger.Log.Debugf("%v %v", token, repo)

	if deleted, ok := g.Search("deleted").Data().(bool); ok && deleted {
		params.Vars["purge"] = "true"
	}
	if created, ok := g.Search("created").Data().(bool); ok && !created {
		if strings.Compare(params.Vars["purge"], "true") != 0 {
			m := false
			commits, _ := g.S("commits").Children()
			for _, commit := range commits {
				modified, _ := commit.S("modified").Children()
				added, _ := commit.S("added").Children()
				for _, f := range append(modified, added...) {
					if strings.Compare(f.Data().(string), manifestName) != -1 {
						m = true
						break
					}
				}
			}
			if !m {
				return fmt.Errorf("%s not change", manifestName)
			}

		}
	}

	if params.Manifest, err = p.uploadGithubManifest(host, token, repo, params.Ref, manifestName); err != nil {
		return err
	}

	p.pusher(e.Trace, []string{"gocd.pipeline.create", "db.create"}, params)
	if strings.Compare(params.Vars["purge"], "true") == 0 {
		p.pusher(e.Trace, []string{"outdated"}, params)
	}
	return nil
}

func (p *manifest) uploadGitlabManifest(host, token, repo, ref, name string) ([]byte, error) {
	git := gitlab.NewClient(nil, token)
	git.SetBaseURL(host)

	gf := &gitlab.GetFileOptions{
		FilePath: &name,
		Ref:      &ref,
	}
	f, _, err := git.RepositoryFiles.GetFile(repo, gf)
	if err != nil {
		return make([]byte, 0), err
	}
	if strings.Compare(f.Encoding, "base64") == 0 {
		return base64.StdEncoding.DecodeString(f.Content)
	} else {
		return make([]byte, 0), fmt.Errorf("Error encoding %v", f.Encoding)
	}
}

func (p *manifest) uploadGithubManifest(host, token, repo, ref, name string) ([]byte, error) {
	logger.Log.Debug(host, token, repo, ref, name)

	resp, err := http.Get(fmt.Sprintf("%s%s?access_token=%s&ref=%s", repo, name, token, ref))
	if err != nil {
		return make([]byte, 0), err
	}

	if resp.StatusCode != 200 {
		return make([]byte, 0), fmt.Errorf("Error code: %v", resp.StatusCode)
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return make([]byte, 0), err
	}

	content := github.RepositoryContent{}
	if err := json.Unmarshal(data, &content); err != nil {
		return make([]byte, 0), err
	}

	if strings.Compare(*content.Encoding, "base64") == 0 {
		return base64.StdEncoding.DecodeString(*content.Content)
	} else {
		return make([]byte, 0), fmt.Errorf("Error encoding %v", content.Encoding)
	}
}
