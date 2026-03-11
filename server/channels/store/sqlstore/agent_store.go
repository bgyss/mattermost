// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"
)

// SqlAgentStore implements store.AgentStore against PostgreSQL.
type SqlAgentStore struct {
	*SqlStore
}

func newSqlAgentStore(sqlStore *SqlStore) store.AgentStore {
	return &SqlAgentStore{SqlStore: sqlStore}
}

// ---------------------------------------------------------------------------
// Workgroup
// ---------------------------------------------------------------------------

func (s *SqlAgentStore) SaveWorkgroup(wg *model.Workgroup) (*model.Workgroup, error) {
	wg.PreSave()
	if err := wg.IsValid(); err != nil {
		return nil, err
	}

	channelIdsJSON, err := json.Marshal(wg.ChannelIds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal ChannelIds")
	}

	_, dbErr := s.GetMaster().NamedExec(`
		INSERT INTO Workgroups
			(Id, TeamId, Name, DisplayName, Type, HeadAgentId, Status, ChannelIds, CreateAt, UpdateAt, DeleteAt)
		VALUES
			(:Id, :TeamId, :Name, :DisplayName, :Type, :HeadAgentId, :Status, :ChannelIds, :CreateAt, :UpdateAt, :DeleteAt)`,
		map[string]interface{}{
			"Id":          wg.Id,
			"TeamId":      wg.TeamId,
			"Name":        wg.Name,
			"DisplayName": wg.DisplayName,
			"Type":        wg.Type,
			"HeadAgentId": wg.HeadAgentId,
			"Status":      wg.Status,
			"ChannelIds":  string(channelIdsJSON),
			"CreateAt":    wg.CreateAt,
			"UpdateAt":    wg.UpdateAt,
			"DeleteAt":    wg.DeleteAt,
		})
	if dbErr != nil {
		return nil, errors.Wrap(dbErr, "failed to save workgroup")
	}
	return wg, nil
}

func (s *SqlAgentStore) UpdateWorkgroup(wg *model.Workgroup) (*model.Workgroup, error) {
	wg.PreUpdate()

	channelIdsJSON, err := json.Marshal(wg.ChannelIds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal ChannelIds")
	}

	_, dbErr := s.GetMaster().NamedExec(`
		UPDATE Workgroups SET
			TeamId=:TeamId, Name=:Name, DisplayName=:DisplayName, Type=:Type,
			HeadAgentId=:HeadAgentId, Status=:Status, ChannelIds=:ChannelIds,
			UpdateAt=:UpdateAt, DeleteAt=:DeleteAt
		WHERE Id=:Id`,
		map[string]interface{}{
			"Id":          wg.Id,
			"TeamId":      wg.TeamId,
			"Name":        wg.Name,
			"DisplayName": wg.DisplayName,
			"Type":        wg.Type,
			"HeadAgentId": wg.HeadAgentId,
			"Status":      wg.Status,
			"ChannelIds":  string(channelIdsJSON),
			"UpdateAt":    wg.UpdateAt,
			"DeleteAt":    wg.DeleteAt,
		})
	if dbErr != nil {
		return nil, errors.Wrap(dbErr, "failed to update workgroup")
	}
	return wg, nil
}

func (s *SqlAgentStore) GetWorkgroup(id string) (*model.Workgroup, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "TeamId", "Name", "DisplayName", "Type", "HeadAgentId", "Status", "ChannelIds", "CreateAt", "UpdateAt", "DeleteAt").
		From("Workgroups").
		Where(sq.Eq{"Id": id, "DeleteAt": 0}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "workgroup_get_tosql")
	}

	row := s.GetReplica().QueryRowX(query, args...)
	return s.scanWorkgroup(row)
}

func (s *SqlAgentStore) GetWorkgroupByName(teamId, name string) (*model.Workgroup, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "TeamId", "Name", "DisplayName", "Type", "HeadAgentId", "Status", "ChannelIds", "CreateAt", "UpdateAt", "DeleteAt").
		From("Workgroups").
		Where(sq.Eq{"TeamId": teamId, "Name": name, "DeleteAt": 0}).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "workgroup_get_by_name_tosql")
	}

	row := s.GetReplica().QueryRowX(query, args...)
	return s.scanWorkgroup(row)
}

func (s *SqlAgentStore) GetWorkgroupsForTeam(teamId string) ([]*model.Workgroup, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "TeamId", "Name", "DisplayName", "Type", "HeadAgentId", "Status", "ChannelIds", "CreateAt", "UpdateAt", "DeleteAt").
		From("Workgroups").
		Where(sq.Eq{"TeamId": teamId, "DeleteAt": 0}).
		OrderBy("CreateAt ASC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "workgroup_list_tosql")
	}

	rows, err := s.GetReplica().QueryX(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list workgroups")
	}
	defer rows.Close()

	var results []*model.Workgroup
	for rows.Next() {
		wg, err := s.scanWorkgroupRows(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, wg)
	}
	return results, rows.Err()
}

func (s *SqlAgentStore) DeleteWorkgroup(id string) error {
	_, err := s.GetMaster().Exec(
		`UPDATE Workgroups SET DeleteAt=$1, UpdateAt=$2 WHERE Id=$3`,
		model.GetMillis(), model.GetMillis(), id,
	)
	return errors.Wrap(err, "failed to delete workgroup")
}

func (s *SqlAgentStore) scanWorkgroup(row interface{ Scan(...interface{}) error }) (*model.Workgroup, error) {
	wg := &model.Workgroup{}
	var channelIdsJSON string
	err := row.Scan(&wg.Id, &wg.TeamId, &wg.Name, &wg.DisplayName, &wg.Type,
		&wg.HeadAgentId, &wg.Status, &channelIdsJSON, &wg.CreateAt, &wg.UpdateAt, &wg.DeleteAt)
	if err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("Workgroup", "unknown")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan workgroup")
	}
	if err = json.Unmarshal([]byte(channelIdsJSON), &wg.ChannelIds); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ChannelIds")
	}
	return wg, nil
}

type rowScanner interface {
	Scan(...interface{}) error
}

func (s *SqlAgentStore) scanWorkgroupRows(rows rowScanner) (*model.Workgroup, error) {
	wg := &model.Workgroup{}
	var channelIdsJSON string
	err := rows.Scan(&wg.Id, &wg.TeamId, &wg.Name, &wg.DisplayName, &wg.Type,
		&wg.HeadAgentId, &wg.Status, &channelIdsJSON, &wg.CreateAt, &wg.UpdateAt, &wg.DeleteAt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan workgroup row")
	}
	if err = json.Unmarshal([]byte(channelIdsJSON), &wg.ChannelIds); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal ChannelIds")
	}
	return wg, nil
}

// ---------------------------------------------------------------------------
// AgentDefinition
// ---------------------------------------------------------------------------

func (s *SqlAgentStore) SaveAgentDefinition(def *model.AgentDefinition) (*model.AgentDefinition, error) {
	def.PreSave()
	if err := def.IsValid(); err != nil {
		return nil, err
	}

	toolsJSON, err := json.Marshal(def.Tools)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal Tools")
	}
	capsJSON, err := json.Marshal(def.Capabilities)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal Capabilities")
	}
	paramsJSON, err := json.Marshal(def.ModelParameters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal ModelParameters")
	}
	memCfgJSON, err := json.Marshal(def.MemoryConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal MemoryConfig")
	}

	_, dbErr := s.GetMaster().NamedExec(`
		INSERT INTO AgentDefinitions
			(Id, WorkgroupId, Role, BotUserId, DisplayName, SystemPrompt, LLMServiceId, ModelId,
			 ModelParameters, Tools, Capabilities, MaxConcurrency, PoolSize, MemoryConfig,
			 OwnerUserId, CreateAt, UpdateAt, DeleteAt)
		VALUES
			(:Id, :WorkgroupId, :Role, :BotUserId, :DisplayName, :SystemPrompt, :LLMServiceId, :ModelId,
			 :ModelParameters, :Tools, :Capabilities, :MaxConcurrency, :PoolSize, :MemoryConfig,
			 :OwnerUserId, :CreateAt, :UpdateAt, :DeleteAt)`,
		map[string]interface{}{
			"Id":              def.Id,
			"WorkgroupId":     def.WorkgroupId,
			"Role":            def.Role,
			"BotUserId":       def.BotUserId,
			"DisplayName":     def.DisplayName,
			"SystemPrompt":    def.SystemPrompt,
			"LLMServiceId":    def.LLMServiceId,
			"ModelId":         def.ModelId,
			"ModelParameters": string(paramsJSON),
			"Tools":           string(toolsJSON),
			"Capabilities":    string(capsJSON),
			"MaxConcurrency":  def.MaxConcurrency,
			"PoolSize":        def.PoolSize,
			"MemoryConfig":    string(memCfgJSON),
			"OwnerUserId":     def.OwnerUserId,
			"CreateAt":        def.CreateAt,
			"UpdateAt":        def.UpdateAt,
			"DeleteAt":        def.DeleteAt,
		})
	if dbErr != nil {
		return nil, errors.Wrap(dbErr, "failed to save agent definition")
	}
	return def, nil
}

func (s *SqlAgentStore) UpdateAgentDefinition(def *model.AgentDefinition) (*model.AgentDefinition, error) {
	def.PreUpdate()

	toolsJSON, _ := json.Marshal(def.Tools)
	capsJSON, _ := json.Marshal(def.Capabilities)
	paramsJSON, _ := json.Marshal(def.ModelParameters)
	memCfgJSON, _ := json.Marshal(def.MemoryConfig)

	_, err := s.GetMaster().NamedExec(`
		UPDATE AgentDefinitions SET
			WorkgroupId=:WorkgroupId, Role=:Role, BotUserId=:BotUserId, DisplayName=:DisplayName,
			SystemPrompt=:SystemPrompt, LLMServiceId=:LLMServiceId, ModelId=:ModelId,
			ModelParameters=:ModelParameters, Tools=:Tools, Capabilities=:Capabilities,
			MaxConcurrency=:MaxConcurrency, PoolSize=:PoolSize, MemoryConfig=:MemoryConfig,
			UpdateAt=:UpdateAt
		WHERE Id=:Id AND DeleteAt=0`,
		map[string]interface{}{
			"Id":              def.Id,
			"WorkgroupId":     def.WorkgroupId,
			"Role":            def.Role,
			"BotUserId":       def.BotUserId,
			"DisplayName":     def.DisplayName,
			"SystemPrompt":    def.SystemPrompt,
			"LLMServiceId":    def.LLMServiceId,
			"ModelId":         def.ModelId,
			"ModelParameters": string(paramsJSON),
			"Tools":           string(toolsJSON),
			"Capabilities":    string(capsJSON),
			"MaxConcurrency":  def.MaxConcurrency,
			"PoolSize":        def.PoolSize,
			"MemoryConfig":    string(memCfgJSON),
			"UpdateAt":        def.UpdateAt,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to update agent definition")
	}
	return def, nil
}

func (s *SqlAgentStore) GetAgentDefinition(id string) (*model.AgentDefinition, error) {
	var row struct {
		Id              string
		WorkgroupId     string
		Role            string
		BotUserId       string
		DisplayName     string
		SystemPrompt    string
		LLMServiceId    string
		ModelId         string
		ModelParameters string
		Tools           string
		Capabilities    string
		MaxConcurrency  int
		PoolSize        int
		MemoryConfig    string
		OwnerUserId     string
		CreateAt        int64
		UpdateAt        int64
		DeleteAt        int64
	}

	err := s.GetReplica().Get(&row,
		`SELECT Id, WorkgroupId, Role, BotUserId, DisplayName, SystemPrompt, LLMServiceId, ModelId,
			ModelParameters, Tools, Capabilities, MaxConcurrency, PoolSize, MemoryConfig,
			OwnerUserId, CreateAt, UpdateAt, DeleteAt
		FROM AgentDefinitions WHERE Id=$1 AND DeleteAt=0`, id)
	if err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("AgentDefinition", id)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent definition")
	}

	return unmarshalAgentDefinition(row.Id, row.WorkgroupId, row.Role, row.BotUserId,
		row.DisplayName, row.SystemPrompt, row.LLMServiceId, row.ModelId,
		row.ModelParameters, row.Tools, row.Capabilities, row.MaxConcurrency,
		row.PoolSize, row.MemoryConfig, row.OwnerUserId, row.CreateAt, row.UpdateAt, row.DeleteAt)
}

func (s *SqlAgentStore) GetAgentDefinitionsByWorkgroup(workgroupId string) ([]*model.AgentDefinition, error) {
	type agentRow struct {
		Id              string
		WorkgroupId     string
		Role            string
		BotUserId       string
		DisplayName     string
		SystemPrompt    string
		LLMServiceId    string
		ModelId         string
		ModelParameters string
		Tools           string
		Capabilities    string
		MaxConcurrency  int
		PoolSize        int
		MemoryConfig    string
		OwnerUserId     string
		CreateAt        int64
		UpdateAt        int64
		DeleteAt        int64
	}

	var rows []agentRow
	err := s.GetReplica().Select(&rows,
		`SELECT Id, WorkgroupId, Role, BotUserId, DisplayName, SystemPrompt, LLMServiceId, ModelId,
			ModelParameters, Tools, Capabilities, MaxConcurrency, PoolSize, MemoryConfig,
			OwnerUserId, CreateAt, UpdateAt, DeleteAt
		FROM AgentDefinitions WHERE WorkgroupId=$1 AND DeleteAt=0 ORDER BY CreateAt ASC`, workgroupId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list agent definitions")
	}

	defs := make([]*model.AgentDefinition, 0, len(rows))
	for _, r := range rows {
		def, err := unmarshalAgentDefinition(r.Id, r.WorkgroupId, r.Role, r.BotUserId,
			r.DisplayName, r.SystemPrompt, r.LLMServiceId, r.ModelId,
			r.ModelParameters, r.Tools, r.Capabilities, r.MaxConcurrency,
			r.PoolSize, r.MemoryConfig, r.OwnerUserId, r.CreateAt, r.UpdateAt, r.DeleteAt)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func (s *SqlAgentStore) GetAgentDefinitionByBotUserId(botUserId string) (*model.AgentDefinition, error) {
	type agentRow struct {
		Id              string
		WorkgroupId     string
		Role            string
		BotUserId       string
		DisplayName     string
		SystemPrompt    string
		LLMServiceId    string
		ModelId         string
		ModelParameters string
		Tools           string
		Capabilities    string
		MaxConcurrency  int
		PoolSize        int
		MemoryConfig    string
		OwnerUserId     string
		CreateAt        int64
		UpdateAt        int64
		DeleteAt        int64
	}

	var row agentRow
	err := s.GetReplica().Get(&row,
		`SELECT Id, WorkgroupId, Role, BotUserId, DisplayName, SystemPrompt, LLMServiceId, ModelId,
			ModelParameters, Tools, Capabilities, MaxConcurrency, PoolSize, MemoryConfig,
			OwnerUserId, CreateAt, UpdateAt, DeleteAt
		FROM AgentDefinitions WHERE BotUserId=$1 AND DeleteAt=0 LIMIT 1`, botUserId)
	if err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("AgentDefinition", botUserId)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent definition by bot user id")
	}

	return unmarshalAgentDefinition(row.Id, row.WorkgroupId, row.Role, row.BotUserId,
		row.DisplayName, row.SystemPrompt, row.LLMServiceId, row.ModelId,
		row.ModelParameters, row.Tools, row.Capabilities, row.MaxConcurrency,
		row.PoolSize, row.MemoryConfig, row.OwnerUserId, row.CreateAt, row.UpdateAt, row.DeleteAt)
}

func (s *SqlAgentStore) ListAllAgentDefinitions() ([]*model.AgentDefinition, error) {
	type agentRow struct {
		Id              string
		WorkgroupId     string
		Role            string
		BotUserId       string
		DisplayName     string
		SystemPrompt    string
		LLMServiceId    string
		ModelId         string
		ModelParameters string
		Tools           string
		Capabilities    string
		MaxConcurrency  int
		PoolSize        int
		MemoryConfig    string
		OwnerUserId     string
		CreateAt        int64
		UpdateAt        int64
		DeleteAt        int64
	}

	var rows []agentRow
	err := s.GetReplica().Select(&rows,
		`SELECT Id, WorkgroupId, Role, BotUserId, DisplayName, SystemPrompt, LLMServiceId, ModelId,
			ModelParameters, Tools, Capabilities, MaxConcurrency, PoolSize, MemoryConfig,
			OwnerUserId, CreateAt, UpdateAt, DeleteAt
		FROM AgentDefinitions WHERE DeleteAt=0 ORDER BY CreateAt ASC`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all agent definitions")
	}

	defs := make([]*model.AgentDefinition, 0, len(rows))
	for _, row := range rows {
		def, err := unmarshalAgentDefinition(row.Id, row.WorkgroupId, row.Role, row.BotUserId,
			row.DisplayName, row.SystemPrompt, row.LLMServiceId, row.ModelId,
			row.ModelParameters, row.Tools, row.Capabilities, row.MaxConcurrency,
			row.PoolSize, row.MemoryConfig, row.OwnerUserId, row.CreateAt, row.UpdateAt, row.DeleteAt)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func (s *SqlAgentStore) DeleteAgentDefinition(id string) error {
	_, err := s.GetMaster().Exec(
		`UPDATE AgentDefinitions SET DeleteAt=$1, UpdateAt=$2 WHERE Id=$3`,
		model.GetMillis(), model.GetMillis(), id,
	)
	return errors.Wrap(err, "failed to delete agent definition")
}

func unmarshalAgentDefinition(
	id, workgroupId, role, botUserId, displayName, systemPrompt, llmServiceId, modelId,
	modelParamsJSON, toolsJSON, capsJSON string, maxConcurrency, poolSize int,
	memCfgJSON, ownerUserId string, createAt, updateAt, deleteAt int64,
) (*model.AgentDefinition, error) {
	def := &model.AgentDefinition{
		Id:             id,
		WorkgroupId:    workgroupId,
		Role:           role,
		BotUserId:      botUserId,
		DisplayName:    displayName,
		SystemPrompt:   systemPrompt,
		LLMServiceId:   llmServiceId,
		ModelId:        modelId,
		MaxConcurrency: maxConcurrency,
		PoolSize:       poolSize,
		OwnerUserId:    ownerUserId,
		CreateAt:       createAt,
		UpdateAt:       updateAt,
		DeleteAt:       deleteAt,
	}
	if err := json.Unmarshal([]byte(modelParamsJSON), &def.ModelParameters); err != nil {
		return nil, fmt.Errorf("unmarshal ModelParameters: %w", err)
	}
	if err := json.Unmarshal([]byte(toolsJSON), &def.Tools); err != nil {
		return nil, fmt.Errorf("unmarshal Tools: %w", err)
	}
	if err := json.Unmarshal([]byte(capsJSON), &def.Capabilities); err != nil {
		return nil, fmt.Errorf("unmarshal Capabilities: %w", err)
	}
	if err := json.Unmarshal([]byte(memCfgJSON), &def.MemoryConfig); err != nil {
		return nil, fmt.Errorf("unmarshal MemoryConfig: %w", err)
	}
	return def, nil
}

// ---------------------------------------------------------------------------
// AgentTask
// ---------------------------------------------------------------------------

func (s *SqlAgentStore) SaveAgentTask(task *model.AgentTask) (*model.AgentTask, error) {
	task.PreSave()
	if err := task.IsValid(); err != nil {
		return nil, err
	}

	chainJSON, _ := json.Marshal(task.DelegationChain)
	inputJSON, _ := json.Marshal(task.Input)
	outputJSON, _ := json.Marshal(task.Output)

	_, err := s.GetMaster().NamedExec(`
		INSERT INTO AgentTasks
			(Id, AgentId, ParentTaskId, RootTaskId, ChannelId, ThreadRootPostId, RequestPostId,
			 ResponsePostId, Input, Output, Status, Priority, DelegationChain, TokensUsed, LatencyMs,
			 ErrorMsg, CreateAt, UpdateAt, ClaimedAt, CompleteAt, ClaimedByServer)
		VALUES
			(:Id, :AgentId, :ParentTaskId, :RootTaskId, :ChannelId, :ThreadRootPostId, :RequestPostId,
			 :ResponsePostId, :Input, :Output, :Status, :Priority, :DelegationChain, :TokensUsed, :LatencyMs,
			 :ErrorMsg, :CreateAt, :UpdateAt, :ClaimedAt, :CompleteAt, :ClaimedByServer)`,
		map[string]interface{}{
			"Id":               task.Id,
			"AgentId":          task.AgentId,
			"ParentTaskId":     task.ParentTaskId,
			"RootTaskId":       task.RootTaskId,
			"ChannelId":        task.ChannelId,
			"ThreadRootPostId": task.ThreadRootPostId,
			"RequestPostId":    task.RequestPostId,
			"ResponsePostId":   task.ResponsePostId,
			"Input":            string(inputJSON),
			"Output":           string(outputJSON),
			"Status":           task.Status,
			"Priority":         task.Priority,
			"DelegationChain":  string(chainJSON),
			"TokensUsed":       task.TokensUsed,
			"LatencyMs":        task.LatencyMs,
			"ErrorMsg":         task.ErrorMsg,
			"CreateAt":         task.CreateAt,
			"UpdateAt":         task.UpdateAt,
			"ClaimedAt":        task.ClaimedAt,
			"CompleteAt":       task.CompleteAt,
			"ClaimedByServer":  task.ClaimedByServer,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to save agent task")
	}
	return task, nil
}

func (s *SqlAgentStore) UpdateAgentTask(task *model.AgentTask) (*model.AgentTask, error) {
	task.UpdateAt = model.GetMillis()

	chainJSON, _ := json.Marshal(task.DelegationChain)
	outputJSON, _ := json.Marshal(task.Output)

	_, err := s.GetMaster().NamedExec(`
		UPDATE AgentTasks SET
			Status=:Status, Output=:Output, DelegationChain=:DelegationChain,
			TokensUsed=:TokensUsed, LatencyMs=:LatencyMs, ErrorMsg=:ErrorMsg,
			ResponsePostId=:ResponsePostId, UpdateAt=:UpdateAt, ClaimedAt=:ClaimedAt,
			CompleteAt=:CompleteAt, ClaimedByServer=:ClaimedByServer
		WHERE Id=:Id`,
		map[string]interface{}{
			"Id":              task.Id,
			"Status":          task.Status,
			"Output":          string(outputJSON),
			"DelegationChain": string(chainJSON),
			"TokensUsed":      task.TokensUsed,
			"LatencyMs":       task.LatencyMs,
			"ErrorMsg":        task.ErrorMsg,
			"ResponsePostId":  task.ResponsePostId,
			"UpdateAt":        task.UpdateAt,
			"ClaimedAt":       task.ClaimedAt,
			"CompleteAt":      task.CompleteAt,
			"ClaimedByServer": task.ClaimedByServer,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to update agent task")
	}
	return task, nil
}

func (s *SqlAgentStore) GetAgentTask(id string) (*model.AgentTask, error) {
	type taskRow struct {
		Id               string
		AgentId          string
		ParentTaskId     string
		RootTaskId       string
		ChannelId        string
		ThreadRootPostId string
		RequestPostId    string
		ResponsePostId   string
		Input            string
		Output           string
		Status           string
		Priority         int
		DelegationChain  string
		TokensUsed       int
		LatencyMs        int64
		ErrorMsg         string
		CreateAt         int64
		UpdateAt         int64
		ClaimedAt        int64
		CompleteAt       int64
		ClaimedByServer  string
	}

	var row taskRow
	err := s.GetReplica().Get(&row,
		`SELECT Id, AgentId, ParentTaskId, RootTaskId, ChannelId, ThreadRootPostId, RequestPostId,
			ResponsePostId, Input, Output, Status, Priority, DelegationChain, TokensUsed, LatencyMs,
			ErrorMsg, CreateAt, UpdateAt, ClaimedAt, CompleteAt, ClaimedByServer
		FROM AgentTasks WHERE Id=$1`, id)
	if err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("AgentTask", id)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get agent task")
	}

	return unmarshalAgentTask(row.Id, row.AgentId, row.ParentTaskId, row.RootTaskId,
		row.ChannelId, row.ThreadRootPostId, row.RequestPostId, row.ResponsePostId,
		row.Input, row.Output, row.Status, row.Priority, row.DelegationChain,
		row.TokensUsed, row.LatencyMs, row.ErrorMsg, row.CreateAt, row.UpdateAt,
		row.ClaimedAt, row.CompleteAt, row.ClaimedByServer)
}

func (s *SqlAgentStore) GetActiveTasksForAgent(agentId string) ([]*model.AgentTask, error) {
	return s.getTasksByAgentAndStatuses(agentId,
		[]string{
			model.AgentTaskStatusPending,
			model.AgentTaskStatusClaimed,
			model.AgentTaskStatusRunning,
			model.AgentTaskStatusAwaitingSubtask,
		})
}

func (s *SqlAgentStore) getTasksByAgentAndStatuses(agentId string, statuses []string) ([]*model.AgentTask, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "AgentId", "ParentTaskId", "RootTaskId", "ChannelId", "ThreadRootPostId",
			"RequestPostId", "ResponsePostId", "Input", "Output", "Status", "Priority",
			"DelegationChain", "TokensUsed", "LatencyMs", "ErrorMsg", "CreateAt", "UpdateAt",
			"ClaimedAt", "CompleteAt", "ClaimedByServer").
		From("AgentTasks").
		Where(sq.Eq{"AgentId": agentId, "Status": statuses}).
		OrderBy("Priority DESC, CreateAt ASC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "agent_tasks_list_tosql")
	}

	rows, err := s.GetReplica().QueryX(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list active tasks")
	}
	defer rows.Close()

	var tasks []*model.AgentTask
	for rows.Next() {
		var r struct {
			Id               string
			AgentId          string
			ParentTaskId     string
			RootTaskId       string
			ChannelId        string
			ThreadRootPostId string
			RequestPostId    string
			ResponsePostId   string
			Input            string
			Output           string
			Status           string
			Priority         int
			DelegationChain  string
			TokensUsed       int
			LatencyMs        int64
			ErrorMsg         string
			CreateAt         int64
			UpdateAt         int64
			ClaimedAt        int64
			CompleteAt       int64
			ClaimedByServer  string
		}
		if err := rows.StructScan(&r); err != nil {
			return nil, errors.Wrap(err, "failed to scan task row")
		}
		task, err := unmarshalAgentTask(r.Id, r.AgentId, r.ParentTaskId, r.RootTaskId,
			r.ChannelId, r.ThreadRootPostId, r.RequestPostId, r.ResponsePostId,
			r.Input, r.Output, r.Status, r.Priority, r.DelegationChain,
			r.TokensUsed, r.LatencyMs, r.ErrorMsg, r.CreateAt, r.UpdateAt,
			r.ClaimedAt, r.CompleteAt, r.ClaimedByServer)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *SqlAgentStore) GetTaskTree(rootTaskId string) ([]*model.AgentTask, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "AgentId", "ParentTaskId", "RootTaskId", "ChannelId", "ThreadRootPostId",
			"RequestPostId", "ResponsePostId", "Input", "Output", "Status", "Priority",
			"DelegationChain", "TokensUsed", "LatencyMs", "ErrorMsg", "CreateAt", "UpdateAt",
			"ClaimedAt", "CompleteAt", "ClaimedByServer").
		From("AgentTasks").
		Where(sq.Eq{"RootTaskId": rootTaskId}).
		OrderBy("CreateAt ASC").
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "task_tree_tosql")
	}

	rows, err := s.GetReplica().QueryX(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task tree")
	}
	defer rows.Close()

	var tasks []*model.AgentTask
	for rows.Next() {
		var r struct {
			Id               string
			AgentId          string
			ParentTaskId     string
			RootTaskId       string
			ChannelId        string
			ThreadRootPostId string
			RequestPostId    string
			ResponsePostId   string
			Input            string
			Output           string
			Status           string
			Priority         int
			DelegationChain  string
			TokensUsed       int
			LatencyMs        int64
			ErrorMsg         string
			CreateAt         int64
			UpdateAt         int64
			ClaimedAt        int64
			CompleteAt       int64
			ClaimedByServer  string
		}
		if err := rows.StructScan(&r); err != nil {
			return nil, errors.Wrap(err, "failed to scan task tree row")
		}
		task, err := unmarshalAgentTask(r.Id, r.AgentId, r.ParentTaskId, r.RootTaskId,
			r.ChannelId, r.ThreadRootPostId, r.RequestPostId, r.ResponsePostId,
			r.Input, r.Output, r.Status, r.Priority, r.DelegationChain,
			r.TokensUsed, r.LatencyMs, r.ErrorMsg, r.CreateAt, r.UpdateAt,
			r.ClaimedAt, r.CompleteAt, r.ClaimedByServer)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// ClaimPendingTask atomically claims the highest-priority pending task for an agent on this server.
func (s *SqlAgentStore) ClaimPendingTask(agentId, serverId string) (*model.AgentTask, error) {
	now := model.GetMillis()

	var taskId string
	err := s.GetMaster().Get(&taskId, `
		UPDATE AgentTasks SET Status=$1, ClaimedAt=$2, ClaimedByServer=$3, UpdateAt=$4
		WHERE Id = (
			SELECT Id FROM AgentTasks
			WHERE AgentId=$5 AND Status=$6
			ORDER BY Priority DESC, CreateAt ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING Id`,
		model.AgentTaskStatusClaimed, now, serverId, now,
		agentId, model.AgentTaskStatusPending)
	if err == sql.ErrNoRows {
		return nil, nil // no pending tasks
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to claim pending task")
	}

	return s.GetAgentTask(taskId)
}

func unmarshalAgentTask(
	id, agentId, parentTaskId, rootTaskId, channelId, threadRootPostId, requestPostId, responsePostId,
	inputJSON, outputJSON, status string, priority int, chainJSON string,
	tokensUsed int, latencyMs int64, errorMsg string,
	createAt, updateAt, claimedAt, completeAt int64, claimedByServer string,
) (*model.AgentTask, error) {
	task := &model.AgentTask{
		Id:               id,
		AgentId:          agentId,
		ParentTaskId:     parentTaskId,
		RootTaskId:       rootTaskId,
		ChannelId:        channelId,
		ThreadRootPostId: threadRootPostId,
		RequestPostId:    requestPostId,
		ResponsePostId:   responsePostId,
		Status:           status,
		Priority:         priority,
		TokensUsed:       tokensUsed,
		LatencyMs:        latencyMs,
		ErrorMsg:         errorMsg,
		CreateAt:         createAt,
		UpdateAt:         updateAt,
		ClaimedAt:        claimedAt,
		CompleteAt:       completeAt,
		ClaimedByServer:  claimedByServer,
	}
	if err := json.Unmarshal([]byte(inputJSON), &task.Input); err != nil {
		return nil, fmt.Errorf("unmarshal AgentTask.Input: %w", err)
	}
	if err := json.Unmarshal([]byte(outputJSON), &task.Output); err != nil {
		return nil, fmt.Errorf("unmarshal AgentTask.Output: %w", err)
	}
	if err := json.Unmarshal([]byte(chainJSON), &task.DelegationChain); err != nil {
		return nil, fmt.Errorf("unmarshal AgentTask.DelegationChain: %w", err)
	}
	return task, nil
}

// ---------------------------------------------------------------------------
// AgentMemory
// ---------------------------------------------------------------------------

func (s *SqlAgentStore) SaveAgentMemory(mem *model.AgentMemory) (*model.AgentMemory, error) {
	mem.PreSave()

	_, err := s.GetMaster().NamedExec(`
		INSERT INTO AgentMemory (Id, AgentId, Scope, ScopeId, Key, ValueText, ExpireAt, CreateAt, UpdateAt)
		VALUES (:Id, :AgentId, :Scope, :ScopeId, :Key, :ValueText, :ExpireAt, :CreateAt, :UpdateAt)
		ON CONFLICT (AgentId, Scope, ScopeId, Key) DO UPDATE SET
			ValueText=EXCLUDED.ValueText, ExpireAt=EXCLUDED.ExpireAt, UpdateAt=EXCLUDED.UpdateAt`,
		map[string]interface{}{
			"Id":        mem.Id,
			"AgentId":   mem.AgentId,
			"Scope":     mem.Scope,
			"ScopeId":   mem.ScopeId,
			"Key":       mem.Key,
			"ValueText": mem.ValueText,
			"ExpireAt":  mem.ExpireAt,
			"CreateAt":  mem.CreateAt,
			"UpdateAt":  mem.UpdateAt,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to save agent memory")
	}
	return mem, nil
}

func (s *SqlAgentStore) GetAgentMemory(agentId, scope, scopeId, key string) (*model.AgentMemory, error) {
	mem := &model.AgentMemory{}
	err := s.GetReplica().Get(mem,
		`SELECT Id, AgentId, Scope, ScopeId, Key, ValueText, ExpireAt, CreateAt, UpdateAt
		FROM AgentMemory WHERE AgentId=$1 AND Scope=$2 AND ScopeId=$3 AND Key=$4`,
		agentId, scope, scopeId, key)
	if err == sql.ErrNoRows {
		return nil, store.NewErrNotFound("AgentMemory", key)
	}
	return mem, errors.Wrap(err, "failed to get agent memory")
}

func (s *SqlAgentStore) GetAgentMemoryByScope(agentId, scope, scopeId string) ([]*model.AgentMemory, error) {
	var mems []*model.AgentMemory
	err := s.GetReplica().Select(&mems,
		`SELECT Id, AgentId, Scope, ScopeId, Key, ValueText, ExpireAt, CreateAt, UpdateAt
		FROM AgentMemory WHERE AgentId=$1 AND Scope=$2 AND ScopeId=$3
		ORDER BY UpdateAt DESC`,
		agentId, scope, scopeId)
	return mems, errors.Wrap(err, "failed to list agent memory")
}

func (s *SqlAgentStore) DeleteAgentMemory(agentId, scope, scopeId string) error {
	_, err := s.GetMaster().Exec(
		`DELETE FROM AgentMemory WHERE AgentId=$1 AND Scope=$2 AND ScopeId=$3`,
		agentId, scope, scopeId)
	return errors.Wrap(err, "failed to delete agent memory")
}

func (s *SqlAgentStore) DeleteExpiredAgentMemory() error {
	now := model.GetMillis()
	_, err := s.GetMaster().Exec(
		`DELETE FROM AgentMemory WHERE ExpireAt > 0 AND ExpireAt < $1`, now)
	return errors.Wrap(err, "failed to delete expired agent memory")
}

// ---------------------------------------------------------------------------
// AgentTaskEvent
// ---------------------------------------------------------------------------

func (s *SqlAgentStore) SaveAgentTaskEvent(event *model.AgentTaskEvent) (*model.AgentTaskEvent, error) {
	event.PreSave()

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal event payload")
	}

	_, err = s.GetMaster().NamedExec(`
		INSERT INTO AgentTaskEvents (Id, TaskId, AgentId, EventType, Payload, CreateAt)
		VALUES (:Id, :TaskId, :AgentId, :EventType, :Payload, :CreateAt)`,
		map[string]interface{}{
			"Id":        event.Id,
			"TaskId":    event.TaskId,
			"AgentId":   event.AgentId,
			"EventType": event.EventType,
			"Payload":   string(payloadJSON),
			"CreateAt":  event.CreateAt,
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to save agent task event")
	}
	return event, nil
}

func (s *SqlAgentStore) GetAgentTaskEvents(taskId string, page, perPage int) ([]*model.AgentTaskEvent, error) {
	query, args, err := s.getQueryBuilder().
		Select("Id", "TaskId", "AgentId", "EventType", "Payload", "CreateAt").
		From("AgentTaskEvents").
		Where(sq.Eq{"TaskId": taskId}).
		OrderBy("CreateAt ASC").
		Limit(uint64(perPage)).
		Offset(uint64(page * perPage)).
		ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "agent_task_events_tosql")
	}

	rows, err := s.GetReplica().QueryX(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get task events")
	}
	defer rows.Close()

	var events []*model.AgentTaskEvent
	for rows.Next() {
		var r struct {
			Id        string
			TaskId    string
			AgentId   string
			EventType string
			Payload   string
			CreateAt  int64
		}
		if err := rows.StructScan(&r); err != nil {
			return nil, errors.Wrap(err, "failed to scan event row")
		}
		e := &model.AgentTaskEvent{
			Id:        r.Id,
			TaskId:    r.TaskId,
			AgentId:   r.AgentId,
			EventType: r.EventType,
			CreateAt:  r.CreateAt,
		}
		if err := json.Unmarshal([]byte(r.Payload), &e.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal event payload: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ---------------------------------------------------------------------------
// Agent coordination channels
// ---------------------------------------------------------------------------

// GetAgentCoordinationChannels returns all ChannelTypeAgentDirect channels for a team.
// These are the observable coordination channels between agent pairs.
func (s *SqlAgentStore) GetAgentCoordinationChannels(teamId string) ([]*model.Channel, error) {
	q := s.getQueryBuilder().
		Select("Id", "TeamId", "Type", "DisplayName", "Name", "Header", "Purpose",
			"LastPostAt", "LastRootPostAt", "TotalMsgCount", "TotalMsgCountRoot",
			"ExtraUpdateAt", "CreatorId", "SchemeId", "Props", "GroupConstrained",
			"Shared", "CreateAt", "UpdateAt", "DeleteAt").
		From("Channels").
		Where(sq.Eq{"TeamId": teamId, "Type": string(model.ChannelTypeAgentDirect), "DeleteAt": 0}).
		OrderBy("CreateAt ASC")

	var channels []*model.Channel
	if err := s.GetReplica().SelectBuilder(&channels, q); err != nil {
		return nil, errors.Wrap(err, "failed to get agent coordination channels")
	}
	return channels, nil
}
