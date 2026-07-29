# Hermes Agent Overview

Hermes Agent is a practical study in building a dependable tool-using agent. This note connects the architecture, execution loop, memory, context management, and tool design into one navigable map.

## Study map

- [Architecture and Runtime](note:{{ARCHITECTURE_ID}})
- [Memory and Context](note:{{MEMORY_ID}})
- [Reliability and Evaluation](note:{{RELIABILITY_ID}})

| Concern | Question |
| --- | --- |
| Runtime | Which component owns each state transition? |
| Context | What information deserves the limited prompt budget? |
| Reliability | How can a failed action be replayed safely? |

The core lesson is simple: useful autonomy comes from explicit state, constrained tools, and observable decisions.
