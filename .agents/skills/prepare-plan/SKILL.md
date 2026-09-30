---
name: prepare-plan
description: Use the skill when the user to asks you to prepare a plan to implement something or do some other work. Usually this is a high level planning phase.
---

# Input

Your current context is the input. If there is not enough information, ask the user for more details.

## Plan Structure

Analyse the desired work and break it down into the following sections:

- **Product Description**: A short product level description of the work to be done. Should include little to no technical details.
- **Reason for the work**: A short explanation of why the work is needed. If it duplicates the product description, skip this section.
- **Alternatives Considered**: Short list of alternatives and why they're discarded. Keep it short and bullet like. If no alternatives discussed, skip this section.
- **Implementation Notes**: High level technical details. Should not be a list of tasks yet, just some technical details that are important for the implementation.
- **Potential Chunks**: 
  - Potential split on chunks of work. 
  - Each chunk is supposed to be self-contained. 
  - Don't think of it as a list of tasks, but rather phases or iterations. 
  - Each such chunk may be a single task or a group of tasks. 
  - Key here is that the chunk is self-contained and small enough to fit the context of a single agent.
  - Don't aim to provide as many chunks as possible. If it's one chunk, that's fine.
- **E2E Testing**: 
  - Any change must be e2e tested
  - This must include checking the directly impacted functionality as well as smoke testing adjacent areas
  - E2E testing may include visual UI checks, API checks e.t.c. Anything that is relevant.
  - Testing should be split on one or multiple steps. Each step may be either isolated or part of the sequence (if makes sense).
  - Description of each step should focus on the test case and expected outcome, but not on the exact execution details.
  - Each testing step should include an expected outcome


## Output

If not otherwise specified, output the plan in a markdown format so the user can easily copy/paste it. If user asks to save it to a file, prefer `tmp/plan-<work-slug>.md` filename, write it to the file and show the file path to the user.


Below plan template must be followed:

```markdown
# Product Description

<product-description>

# Reason for the work

<reason-for-the-work>

# Alternatives Considered

- <alternative-1> - discarded reason
- <alternative-2> - discarded reason
- <alternative-3> - discarded reason

# Implementation Notes

<implementation-notes>

# Potential Chunks

- chunk1 - description
- chunk2 - description
....

# E2E Testing

- **Step 1** 
  * description of what should be tested
  * expected outcome
- **Step 2** 
  * description of what should be tested
  * expected outcome
- ...
```