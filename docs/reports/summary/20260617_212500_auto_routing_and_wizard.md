# Summary: Unified Route Wizard & Auto-Routing Mode

**Timestamp:** 20260617_212500
**Status:** Completed

## What was done
1. Refactored the route creation form in the Routes TUI tab into a step-by-step wizard that guides the user through selecting a primary model and optional fallback models based on their saved provider keys.
2. Implemented an auto-routing engine that analyzes all saved provider keys, classifies them by size/capability tier (large vs fast), ranks them by RPM limits, and generates three optimized routes (`large`, `fast`, and `default`) with complete fallback chains.
3. Updated the footer help text under the Routes TUI tab in `internal/tui/app.go` to display the new key bindings, specifically `[o] auto-optimize`.

## Approach taken
- **Unified Route Wizard**: Used multiple states in the `RoutesModel` state machine (`routesModeWizardPrimary` and `routesModeWizardFallback`) to guide the user sequence. The list of fallback options dynamically excludes keys/providers that are already included in the chain/fallback pool.
- **Auto-Routing (Auto Mode)**: Parsed saved keys, checked their model names for keywords (like `plus`, `pro`, `opus`, `70b`, `120b`, `gpt-4`, `gpt-5`) to partition them, sorted each pool in descending order of RPM limits (fully resolving and prioritizing user overrides, Groq model overrides, and default provider registry RPMs), and saved the generated configs.
- **Dynamic Help Footer**: Edited `internal/tui/app.go` case 2 to present the key bindings clearly to the user.

## Outputs produced
- [routes.go](file:///d:/Codes/oapi/internal/tui/views/routes.go) (modified)
- [app.go](file:///d:/Codes/oapi/internal/tui/app.go) (modified)

## Deviations from PRD
- None. This completes and refines the route configuration and optimization features requested by the user.

## Blockers or open questions
- None.

## Notes for next tasks
- Future enhancements could include tracking real-time status/latencies of providers and incorporating that feedback into the auto-routing mathematical optimizer.
