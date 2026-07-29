# Tool Use and Agent Loop

The Agent Loop alternates between observation, decision, action, and verification. A tool call is never treated as success until its result has been checked.

```go
for state.Runnable() {
	action := planner.Next(state)
	result := tools.Execute(ctx, action)
	state = verifier.Apply(state, action, result)
}
```

If a tool can mutate external state, the loop records an idempotency key before execution. Skills narrow the operating procedure for repeatable tasks; tools provide the actual capability.

See [Architecture and Runtime](note:{{ARCHITECTURE_ID}}) for the component boundary.
