package v1alpha1_test

import (
	"testing"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"

	. "github.com/jonasz-lasut/provider-anthropic/apis/beta/v1alpha1"
)

func TestDeploymentToAnthropicNewCoreFields(t *testing.T) {
	agentID := "agt_1"
	var agentVersion int64 = 3
	envID := "env_1"
	name := "nightly"
	desc := "nightly triage"
	expr := "0 9 * * 1-5"
	tz := "UTC"
	r := &Deployment{
		Spec: DeploymentSpec{ForProvider: DeploymentParameters{
			Name:          &name,
			Description:   &desc,
			AgentID:       &agentID,
			AgentVersion:  &agentVersion,
			EnvironmentID: &envID,
			VaultIDs:      []string{"vlt_1", "vlt_2"},
			Metadata:      map[string]string{"team": "platform"},
			Schedule:      &DeploymentSchedule{Expression: &expr, Timezone: &tz},
		}},
	}
	p := r.ToAnthropicNew(nil)
	if p.Name != "nightly" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.EnvironmentID != "env_1" {
		t.Errorf("EnvironmentID = %q", p.EnvironmentID)
	}
	if p.Description.Value != "nightly triage" {
		t.Errorf("Description = %q", p.Description.Value)
	}
	if p.Agent.OfBetaManagedAgentsAgents == nil {
		t.Fatalf("expected agent object form (id+version)")
	}
	if p.Agent.OfBetaManagedAgentsAgents.ID != "agt_1" || p.Agent.OfBetaManagedAgentsAgents.Version.Value != 3 {
		t.Errorf("agent = %+v", p.Agent.OfBetaManagedAgentsAgents)
	}
	if p.Schedule.Expression != "0 9 * * 1-5" || p.Schedule.Timezone != "UTC" {
		t.Errorf("schedule = %+v", p.Schedule)
	}
	if len(p.VaultIDs) != 2 {
		t.Errorf("VaultIDs = %v", p.VaultIDs)
	}
}

func TestDeploymentToAnthropicNewAgentStringForm(t *testing.T) {
	agentID := "agt_1"
	name := "d"
	envID := "env_1"
	r := &Deployment{Spec: DeploymentSpec{ForProvider: DeploymentParameters{
		Name: &name, AgentID: &agentID, EnvironmentID: &envID,
	}}}
	p := r.ToAnthropicNew(nil)
	if p.Agent.OfString.Value != "agt_1" {
		t.Errorf("expected agent string form, got %+v", p.Agent)
	}
}

func TestDeploymentToAnthropicNewInitialEvents(t *testing.T) {
	name := "d"
	envID := "env_1"
	sysType := "system.message"
	userType := "user.message"
	outcomeType := "user.define_outcome"
	textType := "text"
	imageType := "image"
	base64Type := "base64"
	fileRubric := "file"
	var maxIter int64 = 5
	sysText := "You are an agent."
	userText := "Do the thing."
	imgData := "aGVsbG8="
	imgMedia := "image/png"
	desc := "Produce the report."
	rubricFileID := "file_rubric"
	r := &Deployment{Spec: DeploymentSpec{ForProvider: DeploymentParameters{
		Name: &name, EnvironmentID: &envID,
		InitialEvents: []DeploymentInitialEvent{
			{
				Type:    &sysType,
				Content: []DeploymentContentBlock{{Type: &textType, Text: &sysText}},
			},
			{
				Type: &userType,
				Content: []DeploymentContentBlock{
					{Type: &textType, Text: &userText},
					{Type: &imageType, Source: &DeploymentBlockSource{Type: &base64Type, Data: &imgData, MediaType: &imgMedia}},
				},
			},
			{
				Type:          &outcomeType,
				Description:   &desc,
				MaxIterations: &maxIter,
				Rubric:        &DeploymentRubric{Type: &fileRubric, FileID: &rubricFileID},
			},
		},
	}}}
	p := r.ToAnthropicNew(nil)
	if len(p.InitialEvents) != 3 {
		t.Fatalf("InitialEvents len = %d", len(p.InitialEvents))
	}
	if p.InitialEvents[0].OfSystemMessage == nil || len(p.InitialEvents[0].OfSystemMessage.Content) != 1 {
		t.Fatalf("event[0] system message = %+v", p.InitialEvents[0])
	}
	if p.InitialEvents[0].OfSystemMessage.Content[0].Text != "You are an agent." {
		t.Errorf("system text = %q", p.InitialEvents[0].OfSystemMessage.Content[0].Text)
	}
	um := p.InitialEvents[1].OfUserMessage
	if um == nil || len(um.Content) != 2 {
		t.Fatalf("event[1] user message = %+v", p.InitialEvents[1])
	}
	if um.Content[0].OfText == nil || um.Content[0].OfText.Text != "Do the thing." {
		t.Errorf("user text block = %+v", um.Content[0])
	}
	if um.Content[1].OfImage == nil || um.Content[1].OfImage.Source.OfBase64 == nil {
		t.Fatalf("user image block = %+v", um.Content[1])
	}
	if um.Content[1].OfImage.Source.OfBase64.Data != "aGVsbG8=" {
		t.Errorf("image data = %q", um.Content[1].OfImage.Source.OfBase64.Data)
	}
	od := p.InitialEvents[2].OfUserDefineOutcome
	if od == nil || od.Description != "Produce the report." || od.MaxIterations.Value != 5 {
		t.Fatalf("define outcome = %+v", od)
	}
	if od.Rubric.OfFile == nil || od.Rubric.OfFile.FileID != "file_rubric" {
		t.Errorf("rubric = %+v", od.Rubric)
	}
}

func TestDeploymentToAnthropicNewResources(t *testing.T) {
	name := "d"
	envID := "env_1"
	resType := "github_repository"
	url := "https://github.com/acme/repo"
	r := &Deployment{Spec: DeploymentSpec{ForProvider: DeploymentParameters{
		Name: &name, EnvironmentID: &envID,
		Resources: []SessionResource{{Type: &resType, URL: &url}},
	}}}
	ctx := &DeploymentConversionContext{ResourceTokens: []string{"ghtok"}}
	p := r.ToAnthropicNew(ctx)
	if len(p.Resources) != 1 || p.Resources[0].OfGitHubRepository == nil {
		t.Fatalf("resources = %+v", p.Resources)
	}
	gh := p.Resources[0].OfGitHubRepository
	if gh.URL != "https://github.com/acme/repo" || gh.AuthorizationToken != "ghtok" {
		t.Errorf("github resource = %+v", gh)
	}
}

func TestDeploymentToAnthropicUpdate(t *testing.T) {
	name := "renamed"
	expr := "0 8 * * *"
	tz := "UTC"
	r := &Deployment{Spec: DeploymentSpec{ForProvider: DeploymentParameters{
		Name:     &name,
		Schedule: &DeploymentSchedule{Expression: &expr, Timezone: &tz},
		VaultIDs: []string{"vlt_9"},
	}}}
	p := r.ToAnthropicUpdate(nil)
	if p.Name.Value != "renamed" {
		t.Errorf("Name = %q", p.Name.Value)
	}
	if p.Schedule.Expression != "0 8 * * *" {
		t.Errorf("schedule = %+v", p.Schedule)
	}
	if len(p.VaultIDs) != 1 || p.VaultIDs[0] != "vlt_9" {
		t.Errorf("VaultIDs = %v", p.VaultIDs)
	}
}

func TestDeploymentFromAnthropicObservation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	resp := anthropic.BetaManagedAgentsDeployment{
		ID:            "dpl_1",
		Name:          "nightly",
		Description:   "desc",
		EnvironmentID: "env_1",
		VaultIDs:      []string{"vlt_1"},
		Metadata:      map[string]string{"k": "v"},
		Status:        anthropic.BetaManagedAgentsDeploymentStatusPaused,
		Agent:         anthropic.BetaManagedAgentsAgentReference{ID: "agt_1", Version: 3},
		Schedule:      anthropic.BetaManagedAgentsSchedule{Expression: "0 9 * * 1-5", Timezone: "UTC"},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r := &Deployment{}
	r.FromAnthropicObservation(resp)

	ap := r.Status.AtProvider
	if ap.ID == nil || *ap.ID != "dpl_1" {
		t.Errorf("ID = %v", ap.ID)
	}
	if ap.Name == nil || *ap.Name != "nightly" {
		t.Errorf("Name = %v", ap.Name)
	}
	if ap.AgentID == nil || *ap.AgentID != "agt_1" {
		t.Errorf("AgentID = %v", ap.AgentID)
	}
	if ap.AgentVersion == nil || *ap.AgentVersion != 3 {
		t.Errorf("AgentVersion = %v", ap.AgentVersion)
	}
	if ap.EnvironmentID == nil || *ap.EnvironmentID != "env_1" {
		t.Errorf("EnvironmentID = %v", ap.EnvironmentID)
	}
	if ap.Status == nil || *ap.Status != "paused" {
		t.Errorf("Status = %v", ap.Status)
	}
	if ap.Paused == nil || *ap.Paused != true {
		t.Errorf("Paused = %v", ap.Paused)
	}
	if ap.Schedule == nil || ap.Schedule.Expression == nil || *ap.Schedule.Expression != "0 9 * * 1-5" {
		t.Errorf("Schedule = %+v", ap.Schedule)
	}
	if ap.ArchivedAt != nil {
		t.Errorf("ArchivedAt should be nil, got %v", ap.ArchivedAt)
	}
}
