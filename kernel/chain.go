package kernel

import (
	"log/slog"
	"sort"

	grpcapiauthn "github.com/lynx-go/grpcapi/authn"
	grpcapiauthz "github.com/lynx-go/grpcapi/authz"
	grpcapiinterceptor "github.com/lynx-go/grpcapi/interceptor"
	grpcapipolicy "github.com/lynx-go/grpcapi/policy"
)

// DefaultChain 构建「标准链 + 模块贡献 Stage」：任何模块贡献拦截器 Stage
// （如 audit 的 Order=50）时，server.New 的默认链会被完全接管——此处把
// 必须复刻的基础段（recovery/requestid/clientinfo/authn/authz/validate）
// 收进 kernel，应用侧一行调用；漏一段基础段的后果是认证与权限判定静默
// 丢失，这类 bug 属于「用户不想改、出错要能远程修」，按 ADR-0003 的分界
// 归 kernel。无 Stage 时返回 (nil, nil)——沿用 server.New 自建默认链。
//
// authnOpts 传应用侧的认证选项（Enrichers 等）；IsAnonymous 由本函数按
// 策略集覆写。AssembleWith 要求 Order 单调不减（sort.SliceStable 保证；
// 模块 Stage 落在 clientinfo 与 authz/validate 之间，见 audit=50）。
func DefaultChain(logger *slog.Logger, set *grpcapipolicy.Set, agg *Aggregated, authnOpts grpcapiauthn.Options) (*grpcapiinterceptor.Chain, error) {
	if agg == nil || len(agg.Stages) == 0 {
		return nil, nil
	}
	opts := authnOpts
	opts.IsAnonymous = set.IsAnonymous
	authenticator := grpcapiauthn.New(grpcapiauthn.Chain(agg.Authenticators), opts)
	decider, err := grpcapiauthz.NewRequirementDecider(set)
	if err != nil {
		return nil, err
	}
	authorizer, err := grpcapiauthz.NewInterceptor(set, decider, grpcapiauthz.InterceptorOptions{})
	if err != nil {
		return nil, err
	}
	stages := append([]grpcapiinterceptor.Stage{
		grpcapiinterceptor.RecoveryStage(grpcapiinterceptor.RecoveryOptions{Logger: logger}),
		grpcapiinterceptor.RequestIDStage(grpcapiinterceptor.RequestIDOptions{Logger: logger}),
		grpcapiinterceptor.ClientInfoStage(grpcapiinterceptor.NewClientInfo(grpcapiinterceptor.ClientInfoConfig{})),
		grpcapiinterceptor.AuthnStage(authenticator.Unary(), authenticator.Stream()),
		grpcapiinterceptor.AuthzStage(authorizer.Unary(), authorizer.Stream()),
		grpcapiinterceptor.ValidateStage(grpcapiinterceptor.NewValidate()),
	}, agg.Stages...)
	sort.SliceStable(stages, func(i, j int) bool { return stages[i].Order < stages[j].Order })
	return grpcapiinterceptor.Assemble(stages...)
}
