# Memory and Context Management

记忆并不是把所有历史都塞进提示词。上下文管理需要选择与当前目标有关的证据，并保留来源、时间和可信度。

For a context budget $B$, selected items should satisfy:

$$
\sum_{i=1}^{n} tokens(item_i) \le B
$$

## Practical layers

1. Working context for the current turn.
2. Durable task state for replay and recovery.
3. Curated knowledge that has been reviewed by the owner.

The runtime should prefer compact summaries, but it must retain stable links back to canonical facts. Continue with [Reliability and Evaluation](note:{{RELIABILITY_ID}}).
