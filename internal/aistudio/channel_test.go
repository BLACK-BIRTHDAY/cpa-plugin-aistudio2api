package aistudio

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// TestChannelSelection 保留文件绑定与专属能力的通道资格
func TestChannelSelection(t *testing.T) {
	pool := NewAccountPool(nil, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground, ChannelBuild})
	for _, item := range []struct {
		selection AccountSelection
		want      []Channel
	}{
		{AccountSelection{ModelID: "gemini", Method: "generateContent"}, []Channel{ChannelPlayground, ChannelBuild}},
		{AccountSelection{ModelID: "gemini", Method: "generateContent", Channel: ChannelBuild}, []Channel{ChannelBuild}},
		{AccountSelection{ModelID: "gemini", Method: "generateContent", ResourceID: "file", Channel: ChannelBuild}, []Channel{ChannelPlayground}},
		{AccountSelection{ModelID: "gemini", Method: "countTokens"}, []Channel{ChannelPlayground, ChannelBuild}},
		{AccountSelection{ModelID: "gemini", Method: "countTokens", Channel: ChannelBuild}, []Channel{ChannelBuild}},
		{AccountSelection{ModelID: "gemini", Method: "generateContent", PlaygroundOnly: true, Channel: ChannelBuild}, []Channel{ChannelPlayground}},
	} {
		if got := pool.selectionChannelsLocked(item.selection); !reflect.DeepEqual(got, item.want) {
			t.Errorf("selection=%+v channels=%v want=%v", item.selection, got, item.want)
		}
	}
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})
	if got := pool.selectionChannelsLocked(AccountSelection{ModelID: "gemini", Method: "generateContent", Channel: ChannelBuild}); len(got) != 0 {
		t.Fatalf("disabled Build channels=%v", got)
	}
}

// TestPreferredChannelPerAccount 验证优先通道按账户生效，账户的优先通道冷却后由同账户其余通道接住
func TestPreferredChannelPerAccount(t *testing.T) {
	model := Model{ID: "gemini", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accounts := []*Account{
		{ID: "a@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "b@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground, ChannelBuild})
	for _, account := range accounts {
		if err := pool.SetBuildCatalog(account.ID, []Model{model}); err != nil {
			t.Fatal(err)
		}
	}
	selection := AccountSelection{ModelID: "gemini", Method: "generateContent", PreferredChannel: ChannelBuild}
	var picks []string
	pick := func() {
		lease, _, err := pool.TryAcquireFor(context.Background(), selection)
		if err != nil || lease == nil {
			t.Fatalf("lease=%v err=%v", lease, err)
		}
		picks = append(picks, lease.Account().ID+"/"+string(lease.Channel()))
		if err := lease.Release(); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		pick()
	}
	if err := pool.MarkCooldownIfGeneration("a@example.com", ChannelCooldownScope(ChannelBuild, "gemini"), 1, time.Now(), time.Now().Add(time.Hour), "fixture"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		pick()
	}
	want := []string{"a@example.com/build", "b@example.com/build", "a@example.com/build", "a@example.com/playground", "b@example.com/build"}
	if !reflect.DeepEqual(picks, want) {
		t.Fatalf("picks=%v want=%v", picks, want)
	}
}

// TestPreferredAccountCoolingStatus_PreferredCoolingAndOtherAvailable 验证首选账户冷却且池内有其他可用账户
func TestPreferredAccountCoolingStatus_PreferredCoolingAndOtherAvailable(t *testing.T) {
	model := Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accounts := []*Account{
		{ID: "a@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "b@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})

	until := time.Now().Add(time.Hour)
	reason := "rate_limited"
	if err := pool.MarkCooldownIfGeneration("a@example.com", ChannelCooldownScope(ChannelPlayground, "gemini-4-argon"), 1, time.Now(), until, reason); err != nil {
		t.Fatal(err)
	}

	selection := AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent"}
	cooling, preferredCooling, otherAvailable := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", selection)

	if !preferredCooling {
		t.Errorf("expected preferredCooling=true, got false")
	}
	if !otherAvailable {
		t.Errorf("expected otherAvailable=true, got false")
	}
	if cooling.Until.IsZero() {
		t.Errorf("expected non-zero cooling.Until")
	}
	if cooling.Reason != reason {
		t.Errorf("expected cooling.Reason=%q, got %q", reason, cooling.Reason)
	}
}

// TestPreferredAccountCoolingStatus_PreferredReady 验证首选账户未冷却时返回 false
func TestPreferredAccountCoolingStatus_PreferredReady(t *testing.T) {
	model := Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accounts := []*Account{
		{ID: "a@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "b@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})

	selection := AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent"}
	cooling, preferredCooling, otherAvailable := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", selection)

	if preferredCooling {
		t.Errorf("expected preferredCooling=false, got true")
	}
	if otherAvailable {
		t.Errorf("expected otherAvailable=false, got true")
	}
	if !cooling.Until.IsZero() {
		t.Errorf("expected zero cooling.Until, got %v", cooling.Until)
	}
}

// TestPreferredAccountCoolingStatus_AllAccountsCooling 验证所有账户均冷却时 preferredCooling=true 但 otherAvailable=false
func TestPreferredAccountCoolingStatus_AllAccountsCooling(t *testing.T) {
	model := Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accounts := []*Account{
		{ID: "a@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "b@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})

	until := time.Now().Add(time.Hour)
	if err := pool.MarkCooldownIfGeneration("a@example.com", ChannelCooldownScope(ChannelPlayground, "gemini-4-argon"), 1, time.Now(), until, "limit A"); err != nil {
		t.Fatal(err)
	}
	if err := pool.MarkCooldownIfGeneration("b@example.com", ChannelCooldownScope(ChannelPlayground, "gemini-4-argon"), 1, time.Now(), until, "limit B"); err != nil {
		t.Fatal(err)
	}

	selection := AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent"}
	cooling, preferredCooling, otherAvailable := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", selection)

	if !preferredCooling {
		t.Errorf("expected preferredCooling=true, got false")
	}
	if otherAvailable {
		t.Errorf("expected otherAvailable=false, got true")
	}
	if cooling.Until.IsZero() {
		t.Errorf("expected non-zero cooling.Until")
	}
	if cooling.Reason != "limit A" {
		t.Errorf("expected cooling.Reason='limit A', got %q", cooling.Reason)
	}
}

// TestPreferredAccountCoolingStatus_ChannelScopeRespectsBuildAndModelAccounts 验证通道作用域、Build 通道与模型限制规则
func TestPreferredAccountCoolingStatus_ChannelScopeRespectsBuildAndModelAccounts(t *testing.T) {
	argonModel := Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	assistantModel := Model{ID: "gemini-4-argon-build", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true, "assistant_route": true}}
	models := []Model{argonModel, assistantModel}

	accounts := []*Account{
		{ID: "a@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: models, modelAccessGeneration: 1},
		{ID: "b@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: models, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)

	// 1. upstream_channels 为 [playground, build]，A 仅在 playground 冷却而 build 可用 -> preferredCooling 应为 false
	pool.SetUpstreamChannels([]Channel{ChannelPlayground, ChannelBuild})
	if err := pool.SetBuildCatalog("a@example.com", []Model{argonModel}); err != nil {
		t.Fatal(err)
	}
	if err := pool.SetBuildCatalog("b@example.com", []Model{argonModel}); err != nil {
		t.Fatal(err)
	}

	if err := pool.MarkCooldownIfGeneration("a@example.com", ChannelCooldownScope(ChannelPlayground, "gemini-4-argon"), 1, time.Now(), time.Now().Add(time.Hour), "playground quota"); err != nil {
		t.Fatal(err)
	}
	_, preferredCooling, otherAvailable := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent"})
	if preferredCooling {
		t.Errorf("expected preferredCooling=false when build is still open, got true")
	}

	// 2. 当 model_accounts 限制模型仅 [A] 可用，B 全局就绪但被 model_accounts 排除 -> otherAvailable 应为 false
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})
	pool.SetModelAccounts(map[string][]string{"gemini-4-argon": {"a@example.com"}})
	_, preferredCooling, otherAvailable = pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent"})
	if !preferredCooling {
		t.Errorf("expected preferredCooling=true, got false")
	}
	if otherAvailable {
		t.Errorf("expected otherAvailable=false due to model_accounts restriction, got true")
	}

	// 3. 当 selection 为 Assistant 模型 (gemini-4-argon-build)，A 在 -build 上冷却而 B 在 -build 上就绪 -> preferredCooling=true, otherAvailable=true
	pool.SetModelAccounts(nil)
	if err := pool.MarkCooldownIfGeneration("a@example.com", ChannelCooldownScope(ChannelAssistant, "gemini-4-argon-build"), 1, time.Now(), time.Now().Add(time.Hour), "assistant limit"); err != nil {
		t.Fatal(err)
	}
	cooling, preferredCooling, otherAvailable := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", AccountSelection{ModelID: "gemini-4-argon-build", Method: "generateContent"})
	if !preferredCooling {
		t.Errorf("expected preferredCooling=true for assistant model, got false")
	}
	if !otherAvailable {
		t.Errorf("expected otherAvailable=true for assistant model, got false")
	}
	if cooling.Reason != "assistant limit" {
		t.Errorf("expected cooling.Reason='assistant limit', got %q", cooling.Reason)
	}
}

// TestPreferredAccountCoolingStatus_EdgeCases 验证边界条件与异常状态处理
func TestPreferredAccountCoolingStatus_EdgeCases(t *testing.T) {
	model := Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accounts := []*Account{
		{ID: "ready@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "disabled@example.com", Config: AccountConfig{Enabled: false}, State: AccountReady, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "auth_required@example.com", Config: AccountConfig{Enabled: true}, State: AccountAuthRequired, Models: []Model{model}, modelAccessGeneration: 1},
		{ID: "no_models@example.com", Config: AccountConfig{Enabled: true}, State: AccountReady, Models: nil, modelAccessGeneration: 1},
	}
	pool := NewAccountPool(accounts, 1)
	pool.SetUpstreamChannels([]Channel{ChannelPlayground})

	// 1. 空账户与空白字符串账户
	for _, id := range []string{"", "   ", "\t\n"} {
		_, cooling, other := pool.PreferredAccountCoolingStatus(context.Background(), id, AccountSelection{ModelID: "gemini-4-argon"})
		if cooling || other {
			t.Errorf("expected false, false for empty accountID %q", id)
		}
	}

	// 2. 未知账户
	_, cooling, other := pool.PreferredAccountCoolingStatus(context.Background(), "unknown@example.com", AccountSelection{ModelID: "gemini-4-argon"})
	if cooling || other {
		t.Errorf("expected false, false for unknown account")
	}

	// 3. 禁用账户
	_, cooling, other = pool.PreferredAccountCoolingStatus(context.Background(), "disabled@example.com", AccountSelection{ModelID: "gemini-4-argon"})
	if cooling || other {
		t.Errorf("expected false, false for disabled account")
	}

	// 4. 非就绪账户
	_, cooling, other = pool.PreferredAccountCoolingStatus(context.Background(), "auth_required@example.com", AccountSelection{ModelID: "gemini-4-argon"})
	if cooling || other {
		t.Errorf("expected false, false for auth_required account")
	}

	// 5. 不支持目标模型的账户
	_, cooling, other = pool.PreferredAccountCoolingStatus(context.Background(), "no_models@example.com", AccountSelection{ModelID: "gemini-4-argon"})
	if cooling || other {
		t.Errorf("expected false, false for unsupported model")
	}

	// 6. 带 models/ 前缀的模型选择正常识别冷却
	if err := pool.MarkCooldownIfGeneration("ready@example.com", ChannelCooldownScope(ChannelPlayground, "gemini-4-argon"), 1, time.Now(), time.Now().Add(time.Hour), "prefix test"); err != nil {
		t.Fatal(err)
	}
	c, cooling, _ := pool.PreferredAccountCoolingStatus(context.Background(), "ready@example.com", AccountSelection{ModelID: "models/gemini-4-argon", Method: "generateContent"})
	if !cooling || c.Reason != "prefix test" {
		t.Errorf("expected cooling=true with prefix test, got %v (%q)", cooling, c.Reason)
	}
}
