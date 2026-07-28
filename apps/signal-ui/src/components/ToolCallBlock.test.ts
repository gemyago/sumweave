import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/svelte'
import ToolCallBlock from './ToolCallBlock.svelte'

describe('ToolCallBlock', () => {
  it('renders a generic tool call and its payload', () => {
    render(ToolCallBlock, {
      props: {
        name: 'workspacefs_read_file',
        args: { path: '/workspace/README.md' },
        response: { content: 'Finance workspace' },
      },
    })

    expect(screen.getByText('workspacefs_read_file')).toBeInTheDocument()
    expect(screen.getByText(/workspace\/README\.md/)).toBeInTheDocument()
    expect(screen.getByText(/Finance workspace/)).toBeInTheDocument()
  })
})
