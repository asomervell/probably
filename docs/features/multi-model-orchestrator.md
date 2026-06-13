# Multi-Model Orchestrator

The Multi-Model Orchestrator is an advanced AI system that intelligently routes and executes LLM (Large Language Model) tasks using multiple models and execution strategies. It provides cost optimization, quality assurance, and flexible execution patterns for AI-powered features.

## Overview

The orchestrator replaces the previous single-model routing system with a unified framework that supports:

- **Multiple LLM providers**: Google (Gemini), xAI (Grok), Groq
- **Execution strategies**: Simple, Escalate, Consensus, Supervisor, Verify
- **Model roles**: Fast, Reasoning, Tool Call, Verifier, Vision, Planner
- **Task types**: Categorization, P2P categorization, Chat, Statement extraction

## Features

### Execution Strategies

#### Simple Strategy

Single model call for straightforward tasks:

- **Use case**: Basic categorization, simple queries
- **Models**: Fast model (cost-effective)
- **When to use**: Standard tasks that don't require complex reasoning
- **Performance**: Fastest response time
- **Cost**: Lowest cost

#### Escalate Strategy

Cost-optimized approach that starts with a fast model and escalates when needed:

- **Use case**: Transaction categorization (default for worker)
- **Flow**: 
  1. Try fast model first
  2. Check confidence threshold
  3. Escalate low-confidence results to reasoning model
  4. Merge results
- **Performance**: Fast for high-confidence cases, slower for complex cases
- **Cost**: Optimized - only uses expensive models when needed
- **Confidence threshold**: Configurable (default: 0.85)

#### Consensus Strategy

Parallel execution with voting for high-accuracy tasks:

- **Use case**: Critical categorization, high-stakes decisions
- **Flow**:
  1. Execute query on multiple models in parallel
  2. Collect results from all models
  3. Compute consensus (majority vote)
  4. Return consensus result with confidence score
- **Performance**: Slower (parallel execution)
- **Cost**: Higher (multiple model calls)
- **Accuracy**: Highest (multiple models agree)
- **Required models**: Configurable (default: 2)

#### Verify Strategy

Generate-check-refine loop for quality assurance:

- **Use case**: Complex categorization, statement extraction
- **Flow**:
  1. Generator model creates initial result
  2. Verifier model checks the result
  3. If issues found, refine and re-check
  4. Repeat until verified or max iterations
- **Performance**: Slower (multiple iterations)
- **Cost**: Higher (multiple model calls)
- **Quality**: High (verified results)
- **Max iterations**: Configurable (default: 3)

#### Supervisor Strategy

Planner-worker pattern for complex multi-step tasks:

- **Use case**: Complex chat queries, multi-part questions
- **Flow**:
  1. Planner model breaks task into subtasks
  2. Worker model executes each subtask
  3. Synthesis model combines results
  4. Return final answer
- **Performance**: Slower (multiple steps)
- **Cost**: Higher (multiple model calls)
- **Capability**: Handles complex, multi-part queries
- **Default**: Used for chat tasks

### Model Roles

Models are assigned roles based on their capabilities:

- **Fast**: Quick, cost-effective responses (e.g., Gemini Flash)
- **Reasoning**: Deep thinking for complex tasks (e.g., Grok-4)
- **Tool Call**: Function calling capable (e.g., Gemini 2.0)
- **Verifier**: Checks other models' work
- **Vision**: Multimodal processing (images, PDFs)
- **Planner**: Task decomposition and planning

### Task Types

The orchestrator handles different types of tasks:

- **Categorization**: Transaction categorization with tools
- **P2P Categorization**: Person-to-person transfer categorization
- **Chat**: Conversational queries with SQL generation
- **Statement Extraction**: PDF/document processing

### Error Handling

Comprehensive error classification and handling:

- **Error types**: Model unavailable, invalid input, timeout, rate limit, permission denied, API error, parse error
- **Retryable errors**: Automatically retried (timeouts, rate limits)
- **Non-retryable errors**: Immediately failed (permission errors, invalid input)
- **Error context**: Full context about model, strategy, and task type
- **Graceful degradation**: Falls back when possible

### Monitoring and Metrics

Built-in metrics collection:

- **Task counts**: Total, success, failed by task type
- **Latency**: Response times per task type
- **Confidence scores**: Average confidence across tasks
- **Escalation tracking**: Number of escalations
- **Model usage**: Which models are used most

## Configuration

### Environment Variables

```bash
# Model Configuration
LLM_DEFAULT_MODEL=google/gemini-2.5-flash
LLM_REASONING_MODEL=xai/grok-4-1
LLM_TOOL_CALL_MODEL=google/gemini-2.0-flash

# Strategy Configuration
LLM_DEFAULT_STRATEGY=escalate          # simple, escalate, consensus, supervisor, verify
LLM_CHAT_STRATEGY=supervisor          # Strategy for chat tasks
LLM_EXTRACTION_STRATEGY=simple        # Strategy for statement extraction

# Escalation Configuration
LLM_ESCALATE_THRESHOLD=0.85           # Confidence threshold for escalation (0.0-1.0)
LLM_CONSENSUS_REQUIRED=2              # Number of models required for consensus
LLM_MAX_VERIFY_ITERATIONS=3          # Max iterations for verify strategy
```

### Strategy Selection

Strategies are selected based on:

- **Task type**: Different strategies for different tasks
- **Configuration**: Default strategy can be overridden
- **Task-specific**: Chat uses supervisor, extraction uses simple
- **Worker default**: Escalate for cost optimization

## Use Cases

### Transaction Categorization

- **Strategy**: Escalate (default)
- **Flow**: Fast model first, escalate low-confidence
- **Benefits**: Cost-optimized, high accuracy for complex cases
- **Performance**: Fast for most transactions

### P2P Categorization

- **Strategy**: Simple (P2P is simpler)
- **Flow**: Single fast model call
- **Benefits**: Fast, cost-effective
- **Performance**: Very fast

### AI Chat Queries

- **Strategy**: Supervisor
- **Flow**: Planner breaks query into steps, worker executes, synthesis combines
- **Benefits**: Handles complex, multi-part questions
- **Performance**: Slower but more capable

### Statement Extraction

- **Strategy**: Simple
- **Flow**: Single model call with vision support
- **Benefits**: Fast document processing
- **Performance**: Fast

## Performance Characteristics

### Response Times

- **Simple**: Fastest (~1-2 seconds)
- **Escalate**: Fast for high-confidence, slower for escalation (~2-5 seconds)
- **Consensus**: Slower due to parallel execution (~3-6 seconds)
- **Verify**: Slower due to iterations (~5-10 seconds)
- **Supervisor**: Slowest due to multiple steps (~10-20 seconds)

### Cost Optimization

- **Simple**: Lowest cost
- **Escalate**: Optimized - only expensive models when needed
- **Consensus**: Higher cost (multiple models)
- **Verify**: Higher cost (multiple iterations)
- **Supervisor**: Highest cost (multiple model calls)

### Accuracy

- **Simple**: Good for straightforward tasks
- **Escalate**: High accuracy (escalates when needed)
- **Consensus**: Highest accuracy (multiple models agree)
- **Verify**: High accuracy (verified results)
- **Supervisor**: High accuracy (structured approach)

## Best Practices

### Strategy Selection

- Use **Simple** for basic, high-volume tasks
- Use **Escalate** for categorization (cost-optimized)
- Use **Consensus** for critical decisions
- Use **Verify** for quality-sensitive tasks
- Use **Supervisor** for complex, multi-step queries

### Model Configuration

- Configure fast models for high-volume tasks
- Use reasoning models for complex categorization
- Enable tool-calling models when tools are needed
- Set up vision models for document processing

### Error Handling

- Monitor error rates by type
- Adjust retry logic for retryable errors
- Handle non-retryable errors gracefully
- Log errors with full context

## Related Features

- **AI Chat**: Uses Supervisor strategy for complex queries
- **Transaction Categorization**: Uses Escalate strategy
- **P2P Categorization**: Uses Simple strategy
- **Statement Extraction**: Uses Simple strategy with vision models
- **AI Insights**: Uses orchestrator for analysis
