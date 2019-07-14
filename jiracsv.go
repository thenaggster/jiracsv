package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"

	"io.bytenix.com/jiracsv/jira"
)

var commandFlags = struct {
	Configuration string
	Profile       string
	Username      string
}{}

func init() {
	flag.StringVar(&commandFlags.Username, "u", "", "Jira username")
	flag.StringVar(&commandFlags.Configuration, "c", "", "Configuration file")
	flag.StringVar(&commandFlags.Profile, "p", "", "Search profile")
}

func writeIssues(w *csv.Writer, component *string, issues []*jira.Issue) {
	for _, i := range issues {
		stories := i.LinkedIssues.FilterNotObsolete()

		if component != nil {
			stories = stories.FilterByComponent(*component)
		}

		doneStories := stories.FilterDone()

		w.Write([]string{
			googleSheetLink(i.Link, i.Key),
			i.Fields.Summary,
			i.Fields.Type.Name,
			i.Fields.Priority.Name,
			i.Fields.Status.Name,
			i.Owner,
			i.QAContact,
			googleSheetMark(i.Approvals.Approved()),
			googleSheetProgressBar(doneStories.Len(), stories.Len()),
			googleSheetProgressBar(doneStories.StoryPoints(), stories.StoryPoints()),
		})
	}
}

func main() {
	flag.Parse()

	if commandFlags.Configuration == "" {
		panic(fmt.Errorf("configuration file not specified"))
	}

	if commandFlags.Profile == "" {
		panic(fmt.Errorf("profile id file not specified"))
	}

	config, err := ReadConfigFile(commandFlags.Configuration)

	if err != nil {
		panic(err)
	}

	profile := config.FindProfile(commandFlags.Profile)

	if profile == nil {
		panic(fmt.Errorf("profile '%s' not found", commandFlags.Profile))
	}

	password := GetPassword("PASSWORD", true)
	jiraClient, err := jira.NewClient(config.Instance.URL, &commandFlags.Username, &password)

	if err != nil {
		panic(err)
	}

	w := csv.NewWriter(os.Stdout)
	w.Comma = '\t'

	componentIssues := NewComponentsCollection()

	for _, c := range profile.Components.Include {
		componentIssues.Add(c)
	}

	fmt.Fprintf(os.Stdout, "JQL = %s\n", profile.JQL)

	issues, err := jiraClient.FindEpics(profile.JQL)

	if err != nil {
		panic(err)
	}

	componentIssues.AddIssues(issues)

	for _, k := range componentIssues.Items {
		skipComponent := false

		for _, c := range profile.Components.Exclude {
			if k.Name == c {
				skipComponent = true
				break
			}
		}

		if skipComponent {
			continue
		}

		w.Write([]string{k.Name})
		writeIssues(w, &k.Name, k.Issues)

		w.Flush()
	}

	w.Write([]string{"[UNASSIGNED]"})
	writeIssues(w, nil, componentIssues.Orphans)

	w.Flush()
}
