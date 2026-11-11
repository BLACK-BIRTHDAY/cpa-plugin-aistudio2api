package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/Mag1cFall/AIStudio2API/internal/aistudio"
)

type dummyNativeWorker struct {
	accountID   string
	runtimeMu   sync.RWMutex
	runtime     any
	reloadMu    sync.Mutex
	operationMu sync.Mutex
	stateMu     sync.RWMutex
	state       aistudio.WorkerState
}

func makeTestWorker(accountID string) *aistudio.NativeWorker {
	worker := &aistudio.NativeWorker{}
	dummy := (*dummyNativeWorker)(unsafe.Pointer(worker))
	dummy.accountID = accountID
	dummy.state = aistudio.WorkerState{
		AccountID: accountID,
		Phase:     aistudio.WorkerReady,
	}
	return worker
}

type testMockService struct {
	onGenerate func(context.Context, aistudio.GenerateRequest) (<-chan aistudio.Event, error)
}

func (m *testMockService) Models(context.Context) ([]aistudio.Model, error) {
	return nil, nil
}

func (m *testMockService) CountTokens(context.Context, aistudio.TokenCountRequest) (aistudio.TokenCount, error) {
	return aistudio.TokenCount{}, nil
}

func (m *testMockService) Generate(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
	if m.onGenerate != nil {
		return m.onGenerate(ctx, req)
	}
	ch := make(chan aistudio.Event, 1)
	ch <- aistudio.Event{Kind: aistudio.EventText, Text: "default response"}
	close(ch)
	return ch, nil
}

func (m *testMockService) RefreshAccountModels(context.Context, string, bool) ([]aistudio.Model, error) {
	return nil, nil
}

func (m *testMockService) RefreshAccountState(context.Context, string, bool) (aistudio.AccountRefresh, error) {
	return aistudio.AccountRefresh{}, nil
}

func (m *testMockService) ReclaimExpiredResources(context.Context) error {
	return nil
}

func (m *testMockService) ReclaimIdleAssistantApplets(context.Context) error {
	return nil
}

func (m *testMockService) DeleteAccountAssistantApplets(context.Context, string) error {
	return nil
}

func (m *testMockService) CachedModels() []aistudio.Model {
	return nil
}

// TestGenerateWithRetry_PreferredCoolingPropagates429WhenOtherAccountAvailable 验证首选账户冷却且池内有其他可用账户时快速失败 429 且不调用备用账户
func TestGenerateWithRetry_PreferredCoolingPropagates429WhenOtherAccountAvailable(t *testing.T) {
	argonModel := aistudio.Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accA := &aistudio.Account{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}
	accB := &aistudio.Account{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelPlayground})

	until := time.Now().Add(10 * time.Minute)
	if err := pool.MarkCooldownIfGeneration("a@example.com", aistudio.ChannelCooldownScope(aistudio.ChannelPlayground, "gemini-4-argon"), 0, time.Now(), until, "quota exceeded"); err != nil {
		t.Fatal(err)
	}

	var bCalled atomic.Bool
	mockSvc := &testMockService{
		onGenerate: func(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			if req.AccountID == "b@example.com" {
				bCalled.Store(true)
			}
			ch := make(chan aistudio.Event, 1)
			ch <- aistudio.Event{Kind: aistudio.EventText, Text: "response"}
			close(ch)
			return ch, nil
		},
	}

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)

	clientCtx, cancelClient := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClient()
	requestCtx := ContextWithPreferredAccountID(context.Background(), "a@example.com")
	req := aistudio.GenerateRequest{ID: "req-cooling-propagate", Model: "gemini-4-argon"}
	dest := make(chan aistudio.Event, 10)
	callCtx, cancelCall := context.WithCancel(requestCtx)

	svc.generateWithRetry(clientCtx, callCtx, cancelCall, time.Now(), req, "", dest)

	event := <-dest
	if event.Kind != aistudio.EventError {
		t.Fatalf("expected EventError, got event kind %v", event.Kind)
	}
	var coolingErr *aistudio.AllCoolingError
	if !errors.As(event.Err, &coolingErr) {
		t.Fatalf("expected *aistudio.AllCoolingError, got %v", event.Err)
	}
	if coolingErr.HTTPStatus() != http.StatusTooManyRequests {
		t.Errorf("expected HTTP status 429, got %d", coolingErr.HTTPStatus())
	}
	if !coolingErr.RetryAt(time.Now()).After(time.Now()) {
		t.Errorf("expected RetryAt > now, got %v", coolingErr.RetryAt(time.Now()))
	}
	if bCalled.Load() {
		t.Errorf("account B was called; expected only fast-fail without touching sibling accounts")
	}
}

// TestGenerateWithRetry_PreferredHits429MidAttemptPropagatesInsteadOfRotating 验证首选账户在首轮尝试中遇到 429 冷却后直接向上抛出 429 而不轮换至同池正常账户
func TestGenerateWithRetry_PreferredHits429MidAttemptPropagatesInsteadOfRotating(t *testing.T) {
	argonModel := aistudio.Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accA := &aistudio.Account{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}
	accB := &aistudio.Account{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelPlayground})

	var aAttempts, bAttempts atomic.Int32
	mockSvc := &testMockService{
		onGenerate: func(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			if req.AccountID == "a@example.com" {
				aAttempts.Add(1)
				return nil, &aistudio.RPCError{
					StatusCode: http.StatusTooManyRequests,
					Message:    "RESOURCE_EXHAUSTED",
					RetryDelay: 30 * time.Second,
				}
			}
			if req.AccountID == "b@example.com" {
				bAttempts.Add(1)
				ch := make(chan aistudio.Event, 1)
				ch <- aistudio.Event{Kind: aistudio.EventText, Text: "from B"}
				close(ch)
				return ch, nil
			}
			return nil, errors.New("unknown account")
		},
	}

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	workerA := makeTestWorker("a@example.com")
	workerMgr.accounts["a@example.com"].worker = workerA
	workerMgr.accounts["a@example.com"].warm.Store(true)
	workerMgr.accounts["a@example.com"].bootstrapModel = "gemini-4-argon"

	workerB := makeTestWorker("b@example.com")
	workerMgr.accounts["b@example.com"].worker = workerB
	workerMgr.accounts["b@example.com"].warm.Store(true)
	workerMgr.accounts["b@example.com"].bootstrapModel = "gemini-4-argon"

	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)

	clientCtx, cancelClient := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClient()
	requestCtx := ContextWithPreferredAccountID(context.Background(), "a@example.com")
	req := aistudio.GenerateRequest{ID: "req-mid-attempt-429", Model: "gemini-4-argon"}
	dest := make(chan aistudio.Event, 10)
	callCtx, cancelCall := context.WithCancel(requestCtx)

	svc.generateWithRetry(clientCtx, callCtx, cancelCall, time.Now(), req, "", dest)

	event := <-dest
	if event.Kind != aistudio.EventError {
		t.Fatalf("expected EventError, got event kind %v", event.Kind)
	}
	var coolingErr *aistudio.AllCoolingError
	if !errors.As(event.Err, &coolingErr) {
		t.Fatalf("expected *aistudio.AllCoolingError, got %v", event.Err)
	}
	if coolingErr.HTTPStatus() != http.StatusTooManyRequests {
		t.Errorf("expected HTTP status 429, got %d", coolingErr.HTTPStatus())
	}
	if aAttempts.Load() != 1 {
		t.Errorf("expected 1 attempt on account A, got %d", aAttempts.Load())
	}
	if bAttempts.Load() != 0 {
		t.Errorf("account B was called %d times; should NEVER have been called on mid-attempt quota failure", bAttempts.Load())
	}
}

// TestGenerateWithRetry_AllPlaygroundCoolingWaterfallsToAssistantSticky 验证全池 Playground 冷却时正常级联至 Assistant 通道并保持首选账户粘性
func TestGenerateWithRetry_AllPlaygroundCoolingWaterfallsToAssistantSticky(t *testing.T) {
	argonModel := aistudio.Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	argonBuildModel := aistudio.Model{ID: "gemini-4-argon-build", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true, "assistant_route": true}}
	models := []aistudio.Model{argonModel, argonBuildModel}

	accA := &aistudio.Account{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"}, State: aistudio.AccountReady, Models: models}
	accB := &aistudio.Account{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"}, State: aistudio.AccountReady, Models: models}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelPlayground, aistudio.ChannelBuild})
	if err := pool.SetBuildCatalog("a@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}
	if err := pool.SetBuildCatalog("b@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}

	until := time.Now().Add(time.Hour)
	if err := pool.MarkCooldownIfGeneration("a@example.com", aistudio.ChannelCooldownScope(aistudio.ChannelPlayground, "gemini-4-argon"), 0, time.Now(), until, "quota A"); err != nil {
		t.Fatal(err)
	}
	if err := pool.MarkCooldownIfGeneration("b@example.com", aistudio.ChannelCooldownScope(aistudio.ChannelPlayground, "gemini-4-argon"), 0, time.Now(), until, "quota B"); err != nil {
		t.Fatal(err)
	}

	var calledModel, calledAccount string
	mockSvc := &testMockService{
		onGenerate: func(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			calledModel = req.Model
			calledAccount = req.AccountID
			ch := make(chan aistudio.Event, 2)
			ch <- aistudio.Event{Kind: aistudio.EventText, Text: "build content"}
			close(ch)
			return ch, nil
		},
	}

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	workerA := makeTestWorker("a@example.com")
	workerMgr.accounts["a@example.com"].worker = workerA
	workerMgr.accounts["a@example.com"].warm.Store(true)
	workerMgr.accounts["a@example.com"].bootstrapModel = "gemini-4-argon"

	workerB := makeTestWorker("b@example.com")
	workerMgr.accounts["b@example.com"].worker = workerB
	workerMgr.accounts["b@example.com"].warm.Store(true)
	workerMgr.accounts["b@example.com"].bootstrapModel = "gemini-4-argon"

	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)
	svc.assistantFallback = true

	clientCtx, cancelClient := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClient()
	requestCtx := ContextWithPreferredAccountID(context.Background(), "a@example.com")
	req := aistudio.GenerateRequest{ID: "req-waterfall-assistant", Model: "gemini-4-argon"}
	dest := make(chan aistudio.Event, 10)
	callCtx, cancelCall := context.WithCancel(requestCtx)

	svc.generateWithRetry(clientCtx, callCtx, cancelCall, time.Now(), req, "", dest)

	event := <-dest
	if event.Kind != aistudio.EventText {
		t.Fatalf("expected EventText, got kind %v (err=%v)", event.Kind, event.Err)
	}
	if calledAccount != "a@example.com" {
		t.Errorf("expected sticky waterfall to account A, got %s", calledAccount)
	}
	if !strings.HasSuffix(calledModel, "gemini-4-argon-build") {
		t.Errorf("expected -build model, got %s", calledModel)
	}
}

// TestAcquireWarmLease_PromotesStandbyPreferredOverWarmSibling 验证 Standby 状态的首选账户优先提升，不盲目抢占 Warm 的兄弟账户，并在启动中或失败时正确退避/降级
func TestAcquireWarmLease_PromotesStandbyPreferredOverWarmSibling(t *testing.T) {
	argonModel := aistudio.Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accA := &aistudio.Account{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}
	accB := &aistudio.Account{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelPlayground})

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	// B 是 Warm
	workerB := makeTestWorker("b@example.com")
	workerMgr.accounts["b@example.com"].worker = workerB
	workerMgr.accounts["b@example.com"].warm.Store(true)
	workerMgr.accounts["b@example.com"].bootstrapModel = "gemini-4-argon"

	// A 是 StandbyReady (warm.Store(false))
	workerA := makeTestWorker("a@example.com")
	workerMgr.accounts["a@example.com"].worker = workerA
	workerMgr.accounts["a@example.com"].warm.Store(false)
	workerMgr.accounts["a@example.com"].bootstrapModel = "gemini-4-argon"

	mockSvc := &testMockService{}
	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)

	// 1. 验证 A 从 StandbyReady 优先提升，而不使用已就绪的 B
	selection := aistudio.AccountSelection{ModelID: "gemini-4-argon", Method: "generateContent", PreferredAccountID: "a@example.com"}
	lease, err := svc.acquireWarmLease(context.Background(), selection, false)
	if err != nil {
		t.Fatalf("acquireWarmLease error: %v", err)
	}
	if lease.Account().ID != "a@example.com" {
		t.Fatalf("expected preferred account A to be promoted, got %s", lease.Account().ID)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}

	// 2. 验证 preferredOpening 时不会绕过 A 抢占 B，而是等待 A 启动完成
	workerMgr.accounts["a@example.com"].warm.Store(false)
	openingChan := make(chan struct{})
	workerMgr.rebalanceMu.Lock()
	workerMgr.openings["a@example.com"] = openingChan
	workerMgr.rebalanceMu.Unlock()

	go func() {
		time.Sleep(50 * time.Millisecond)
		workerMgr.rebalanceMu.Lock()
		delete(workerMgr.openings, "a@example.com")
		close(openingChan)
		workerMgr.rebalanceMu.Unlock()
		workerMgr.notifyScheduling()
	}()

	leaseOpening, err := svc.acquireWarmLease(context.Background(), selection, false)
	if err != nil {
		t.Fatalf("acquireWarmLease with opening error: %v", err)
	}
	if leaseOpening.Account().ID != "a@example.com" {
		t.Fatalf("expected opening A to be acquired after opening completes, got %s", leaseOpening.Account().ID)
	}
	if err := leaseOpening.Release(); err != nil {
		t.Fatal(err)
	}

	// 3. 验证 A 提升失败（Worker 初始化失败）时，降级使用 WarmReady 的 B
	workerMgr.accounts["a@example.com"].worker = nil
	workerMgr.accounts["a@example.com"].warm.Store(false)
	workerMgr.accounts["a@example.com"].bootstrapModel = "incompatible-model"

	leaseFallback, err := svc.acquireWarmLease(context.Background(), selection, false)
	if err != nil {
		t.Fatalf("acquireWarmLease fallback error: %v", err)
	}
	if leaseFallback.Account().ID != "b@example.com" {
		t.Fatalf("expected fallback to warm B when A fails, got %s", leaseFallback.Account().ID)
	}
	if err := leaseFallback.Release(); err != nil {
		t.Fatal(err)
	}
}

// TestHedgeSelections_PreferredAccountUsesSameAccountOnly 验证指定 PreferredAccountID 时只同账户对冲，不扩散到其他账户
func TestHedgeSelections_PreferredAccountUsesSameAccountOnly(t *testing.T) {
	argonModel := aistudio.Model{ID: "gemini-4-argon", Methods: []string{"generateContent"}, Capabilities: map[string]bool{"chat_model": true}}
	accA := &aistudio.Account{ID: "a@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}
	accB := &aistudio.Account{ID: "b@example.com", Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"}, State: aistudio.AccountReady, Models: []aistudio.Model{argonModel}}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	svc := newTrackedService(context.Background(), &testMockService{}, pool, reqReg, workerMgr, time.Minute)

	// 1. PreferredAccountID 不为空时 preferSameOnly 为 true，仅返回单一同账户选项
	primaryWithPref := aistudio.AccountSelection{ModelID: "gemini-4-argon", PreferredAccountID: "a@example.com"}
	preferSameOnly := strings.TrimSpace(primaryWithPref.PreferredAccountID) != ""
	gotSameOnly := svc.hedgeSelections(primaryWithPref, "a@example.com", preferSameOnly, make(map[string]struct{}))

	if len(gotSameOnly) != 1 {
		t.Fatalf("expected 1 selection for preferred account, got %d", len(gotSameOnly))
	}
	if gotSameOnly[0].AccountID != "a@example.com" {
		t.Errorf("expected same account a@example.com, got %s", gotSameOnly[0].AccountID)
	}
	if len(gotSameOnly[0].AllowedAccountIDs) != 0 {
		t.Errorf("expected nil AllowedAccountIDs, got %v", gotSameOnly[0].AllowedAccountIDs)
	}

	// 2. PreferredAccountID 为空且未指定账户/文件时，先尝试其他账户再尝试同账户
	primaryEmpty := aistudio.AccountSelection{ModelID: "gemini-4-argon"}
	gotMulti := svc.hedgeSelections(primaryEmpty, "a@example.com", false, make(map[string]struct{}))

	if len(gotMulti) != 2 {
		t.Fatalf("expected 2 selections for unconstrained primary, got %d", len(gotMulti))
	}
	if !slices.Contains(gotMulti[0].AllowedAccountIDs, "b@example.com") {
		t.Errorf("expected first selection to target other accounts (b@example.com), got %v", gotMulti[0].AllowedAccountIDs)
	}
	if gotMulti[1].AccountID != "a@example.com" {
		t.Errorf("expected second selection to fallback to same account, got %s", gotMulti[1].AccountID)
	}
}

// TestGenerateWithRetry_BuildFirstEventStallTimeoutSwitchesAccountAndUnbindsSticky 验证 -build 模型首事件等待超时时取消账户 A、解绑首选粘性、切换至账户 B 且不对账户 A 施加池冷却
func TestGenerateWithRetry_BuildFirstEventStallTimeoutSwitchesAccountAndUnbindsSticky(t *testing.T) {
	argonBuildModel := aistudio.Model{
		ID:           "gemini-4-argon-build",
		Methods:      []string{"generateContent"},
		Capabilities: map[string]bool{"chat_model": true, "assistant_route": true},
	}
	accA := &aistudio.Account{
		ID:     "a@example.com",
		Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"},
		State:  aistudio.AccountReady,
		Models: []aistudio.Model{argonBuildModel},
	}
	accB := &aistudio.Account{
		ID:     "b@example.com",
		Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"},
		State:  aistudio.AccountReady,
		Models: []aistudio.Model{argonBuildModel},
	}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelBuild})
	if err := pool.SetBuildCatalog("a@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}
	if err := pool.SetBuildCatalog("b@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}

	var aCalled, bCalled, aCancelled atomic.Bool
	mockSvc := &testMockService{
		onGenerate: func(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			if req.AccountID == "a@example.com" {
				aCalled.Store(true)
				ch := make(chan aistudio.Event)
				go func() {
					<-ctx.Done()
					aCancelled.Store(true)
					close(ch)
				}()
				return ch, nil
			}
			if req.AccountID == "b@example.com" {
				bCalled.Store(true)
				ch := make(chan aistudio.Event, 2)
				ch <- aistudio.Event{Kind: aistudio.EventText, Text: "from B", ProviderModel: "models/gemini-4-argon-build"}
				ch <- aistudio.Event{Kind: aistudio.EventFinish, FinishReason: "STOP"}
				close(ch)
				return ch, nil
			}
			return nil, errors.New("unknown account")
		},
	}

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	workerA := makeTestWorker("a@example.com")
	workerMgr.accounts["a@example.com"].worker = workerA
	workerMgr.accounts["a@example.com"].warm.Store(true)
	workerMgr.accounts["a@example.com"].bootstrapModel = "gemini-4-argon-build"

	workerB := makeTestWorker("b@example.com")
	workerMgr.accounts["b@example.com"].worker = workerB
	workerMgr.accounts["b@example.com"].warm.Store(true)
	workerMgr.accounts["b@example.com"].bootstrapModel = "gemini-4-argon-build"

	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)
	svc.SetBuildStallTimeout(40 * time.Millisecond)

	clientCtx, cancelClient := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClient()
	requestCtx := ContextWithPreferredAccountID(context.Background(), "a@example.com")
	req := aistudio.GenerateRequest{ID: "req-first-event-stall", Model: "gemini-4-argon-build"}
	dest := make(chan aistudio.Event, 10)
	callCtx, cancelCall := context.WithCancel(requestCtx)

	svc.generateWithRetry(clientCtx, callCtx, cancelCall, time.Now(), req, "", dest)

	var receivedEvents []aistudio.Event
	for ev := range dest {
		receivedEvents = append(receivedEvents, ev)
	}

	if !aCalled.Load() {
		t.Errorf("expected account A to be called first")
	}
	if !aCancelled.Load() {
		t.Errorf("expected account A attempt context to be cancelled on stall timeout")
	}
	if !bCalled.Load() {
		t.Errorf("expected failover to account B")
	}

	// Verify Account A is NOT in pool cooldown
	cooling, prefCooling, _ := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", aistudio.AccountSelection{
		ModelID: "gemini-4-argon-build",
		Channel: aistudio.ChannelBuild,
	})
	if prefCooling || !cooling.Until.IsZero() {
		t.Errorf("expected account A NOT to be placed in pool cooldown, got until=%v reason=%s", cooling.Until, cooling.Reason)
	}

	hasText := false
	hasFinish := false
	for _, ev := range receivedEvents {
		if ev.Kind == aistudio.EventError {
			t.Fatalf("unexpected error event: %v", ev.Err)
		}
		if ev.Kind == aistudio.EventText && ev.Text == "from B" {
			hasText = true
		}
		if ev.Kind == aistudio.EventFinish {
			hasFinish = true
		}
	}
	if !hasText {
		t.Errorf("expected EventText from B, got %v", receivedEvents)
	}
	if !hasFinish {
		t.Errorf("expected EventFinish from B, got %v", receivedEvents)
	}
}

// TestGenerateWithRetry_BuildPreCommitStreamingStallTimeoutSeamlessFailover 验证 -build 模型在 EventReasoning 后流式停顿超时时取消账户 A，无缝切换至账户 B 在同一目标流中输出
func TestGenerateWithRetry_BuildPreCommitStreamingStallTimeoutSeamlessFailover(t *testing.T) {
	argonBuildModel := aistudio.Model{
		ID:           "gemini-4-argon-build",
		Methods:      []string{"generateContent"},
		Capabilities: map[string]bool{"chat_model": true, "assistant_route": true},
	}
	accA := &aistudio.Account{
		ID:     "a@example.com",
		Config: aistudio.AccountConfig{Enabled: true, Label: "Account A"},
		State:  aistudio.AccountReady,
		Models: []aistudio.Model{argonBuildModel},
	}
	accB := &aistudio.Account{
		ID:     "b@example.com",
		Config: aistudio.AccountConfig{Enabled: true, Label: "Account B"},
		State:  aistudio.AccountReady,
		Models: []aistudio.Model{argonBuildModel},
	}

	pool := aistudio.NewAccountPool([]*aistudio.Account{accA, accB}, 1)
	pool.SetUpstreamChannels([]aistudio.Channel{aistudio.ChannelBuild})
	if err := pool.SetBuildCatalog("a@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}
	if err := pool.SetBuildCatalog("b@example.com", []aistudio.Model{argonBuildModel}); err != nil {
		t.Fatal(err)
	}

	var aCalled, bCalled, aCancelled atomic.Bool
	mockSvc := &testMockService{
		onGenerate: func(ctx context.Context, req aistudio.GenerateRequest) (<-chan aistudio.Event, error) {
			if req.AccountID == "a@example.com" {
				aCalled.Store(true)
				ch := make(chan aistudio.Event, 5)
				ch <- aistudio.Event{Kind: aistudio.EventReasoning, Text: "reasoning from A", ProviderModel: "models/gemini-4-argon-build"}
				go func() {
					<-ctx.Done()
					aCancelled.Store(true)
					close(ch)
				}()
				return ch, nil
			}
			if req.AccountID == "b@example.com" {
				bCalled.Store(true)
				ch := make(chan aistudio.Event, 5)
				ch <- aistudio.Event{Kind: aistudio.EventReasoning, Text: "reasoning from B", ProviderModel: "models/gemini-4-argon-build"}
				ch <- aistudio.Event{Kind: aistudio.EventText, Text: "text from B", ProviderModel: "models/gemini-4-argon-build"}
				ch <- aistudio.Event{Kind: aistudio.EventFinish, FinishReason: "STOP"}
				close(ch)
				return ch, nil
			}
			return nil, errors.New("unknown account")
		},
	}

	reqReg := newRequestRegistry(context.Background())
	workerMgr := newAccountWorkerManager(pool, []*aistudio.Account{accA, accB}, reqReg, "", "", time.Minute, 1, 2, 1)
	defer workerMgr.cancel()

	workerA := makeTestWorker("a@example.com")
	workerMgr.accounts["a@example.com"].worker = workerA
	workerMgr.accounts["a@example.com"].warm.Store(true)
	workerMgr.accounts["a@example.com"].bootstrapModel = "gemini-4-argon-build"

	workerB := makeTestWorker("b@example.com")
	workerMgr.accounts["b@example.com"].worker = workerB
	workerMgr.accounts["b@example.com"].warm.Store(true)
	workerMgr.accounts["b@example.com"].bootstrapModel = "gemini-4-argon-build"

	svc := newTrackedService(context.Background(), mockSvc, pool, reqReg, workerMgr, time.Minute)
	svc.proxies = aistudio.NewProxyPool(pool, nil)
	svc.SetBuildStallTimeout(40 * time.Millisecond)

	clientCtx, cancelClient := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelClient()
	requestCtx := ContextWithPreferredAccountID(context.Background(), "a@example.com")
	req := aistudio.GenerateRequest{ID: "req-streaming-stall", Model: "gemini-4-argon-build", Unary: false}
	dest := make(chan aistudio.Event, 10)
	callCtx, cancelCall := context.WithCancel(requestCtx)

	svc.generateWithRetry(clientCtx, callCtx, cancelCall, time.Now(), req, "", dest)

	var receivedEvents []aistudio.Event
	for ev := range dest {
		receivedEvents = append(receivedEvents, ev)
	}

	if !aCalled.Load() {
		t.Errorf("expected account A to be called first")
	}
	if !aCancelled.Load() {
		t.Errorf("expected account A attempt context to be cancelled after pre-commit stall")
	}
	if !bCalled.Load() {
		t.Errorf("expected failover to account B")
	}

	// Verify Account A is NOT in pool cooldown
	cooling, prefCooling, _ := pool.PreferredAccountCoolingStatus(context.Background(), "a@example.com", aistudio.AccountSelection{
		ModelID: "gemini-4-argon-build",
		Channel: aistudio.ChannelBuild,
	})
	if prefCooling || !cooling.Until.IsZero() {
		t.Errorf("expected account A NOT to be placed in pool cooldown, got until=%v reason=%s", cooling.Until, cooling.Reason)
	}

	if len(receivedEvents) != 4 {
		t.Fatalf("expected 4 events, got %d: %+v", len(receivedEvents), receivedEvents)
	}
	if receivedEvents[0].Kind != aistudio.EventReasoning || receivedEvents[0].Text != "reasoning from A" {
		t.Errorf("event 0 mismatch: %+v", receivedEvents[0])
	}
	if receivedEvents[1].Kind != aistudio.EventReasoning || receivedEvents[1].Text != "reasoning from B" {
		t.Errorf("event 1 mismatch: %+v", receivedEvents[1])
	}
	if receivedEvents[2].Kind != aistudio.EventText || receivedEvents[2].Text != "text from B" {
		t.Errorf("event 2 mismatch: %+v", receivedEvents[2])
	}
	if receivedEvents[3].Kind != aistudio.EventFinish {
		t.Errorf("event 3 mismatch: %+v", receivedEvents[3])
	}
}
