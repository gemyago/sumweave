# Start OpenSpec Manager

This prompt is used to start some implementation of the work requested by the user.

## Input

The input may vary and can be:
- Verbal description of the work to be done
- Notion ticket reference
- Notion document reference
- Any other reference to the document or task to be implemented

## Process

Your job is exactly this:
1. Do a quick shallow analysis of the intent, just enough so you could create a relevant feature branch for this work
2. Checkout the repository on main branch, pull latest changes
3. Create a new feature branch for the work based on the analysis:
  - keep feature branch name short (within 20-25 characters)
  - use relevant prefix (feat/, fix/, chore/, etc.)
4. Initiate the implementation of the work (see below)

## Initiating the implementation

The user may specify which flow to use. If unsure, use the default implementation flow

### Default implementation flow

If not otherwise specified, follow the [openspec manager](openspec-manager.md) prompt to start the implementation.

### Simple implementation flow

Apply if user says something like:
- This is a simple change
- This should be a quick fix

In this case implement the work directly in a current agent session. Analyse the intent, complete the work, using repository standards to validate the completion, commit the changes and signal the completion to the user.