import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '../../test/test-utils';
import { StreamView, StreamEvent } from '../StreamView';

const mockEvent = (type: StreamEvent['type'], content: string, overrides: Partial<StreamEvent> = {}): StreamEvent => ({
  type,
  content,
  timestamp: new Date().toISOString(),
  ...overrides,
});

describe('StreamView', () => {
  it('renders empty state when no events', () => {
    render(<StreamView events={[]} isRunning={false} />);
    expect(screen.getByText('Waiting for events...')).toBeInTheDocument();
  });

  it('shows event count', () => {
    const events = [
      mockEvent('text', 'Hello'),
      mockEvent('text', 'World'),
    ];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('2 events')).toBeInTheDocument();
  });

  it('renders text events', () => {
    const events = [mockEvent('text', 'Test message')];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Test message')).toBeInTheDocument();
  });

  it('renders error events with error styling', () => {
    const events = [mockEvent('error', 'Something went wrong')];
    const { container } = render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    expect(screen.getByText('Error')).toBeInTheDocument();
    // Check for error icon
    expect(container.textContent).toContain('❌');
  });

  it('renders tool_call events', () => {
    const events = [
      mockEvent('tool_call', '', {
        tool_name: 'shell_exec',
        tool_input: '{"command": "ls -la"}',
      }),
    ];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('shell_exec')).toBeInTheDocument();
  });

  it('renders tool_result events', () => {
    const events = [
      mockEvent('tool_result', 'Success', {
        tool_output: 'file1.txt\nfile2.txt',
      }),
    ];
    const { container } = render(<StreamView events={events} isRunning={false} />);
    // Result should show success indicator
    expect(container.textContent).toContain('✓');
  });

  it('renders reasoning events with italic styling', () => {
    const events = [mockEvent('reasoning', 'I need to think about this...')];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('I need to think about this...')).toBeInTheDocument();
    // Check for thinking emoji
    expect(screen.getByText('💭')).toBeInTheDocument();
  });

  it('renders thinking events same as reasoning', () => {
    const events = [mockEvent('thinking', 'Let me consider the options...')];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Let me consider the options...')).toBeInTheDocument();
    expect(screen.getByText('💭')).toBeInTheDocument();
  });

  it('renders status events', () => {
    const events = [
      mockEvent('status', 'Starting execution...', { status: 'starting' }),
    ];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Starting execution...')).toBeInTheDocument();
  });

  it('renders done events with completion message', () => {
    const events = [mockEvent('done', 'Task completed successfully')];
    const { container } = render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Execution Complete')).toBeInTheDocument();
    expect(container.textContent).toContain('🎉');
  });

  it('renders output events (agent response)', () => {
    const events = [mockEvent('output', 'Here is my response to your question.')];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Agent Response')).toBeInTheDocument();
    expect(screen.getByText('Here is my response to your question.')).toBeInTheDocument();
  });

  it('renders plan events', () => {
    const events = [
      mockEvent('plan', '1. First step\n2. Second step\n3. Third step'),
    ];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('Execution Plan')).toBeInTheDocument();
  });

  it('shows running indicator when isRunning is true', () => {
    const events = [mockEvent('status', 'Processing...', { status: 'running' })];
    render(<StreamView events={events} isRunning={true} />);
    expect(screen.getByText('Execution in progress...')).toBeInTheDocument();
  });

  it('does not show running indicator when isRunning is false', () => {
    const events = [mockEvent('done', 'Complete')];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.queryByText('Execution in progress...')).not.toBeInTheDocument();
  });

  it('shows clear button when onClear is provided', () => {
    const onClear = vi.fn();
    render(<StreamView events={[mockEvent('text', 'test')]} isRunning={false} onClear={onClear} />);
    expect(screen.getByText('Clear')).toBeInTheDocument();
  });

  it('does not show clear button when onClear is not provided', () => {
    render(<StreamView events={[mockEvent('text', 'test')]} isRunning={false} />);
    expect(screen.queryByText('Clear')).not.toBeInTheDocument();
  });

  it('calls onClear when clear button is clicked', () => {
    const onClear = vi.fn();
    render(<StreamView events={[mockEvent('text', 'test')]} isRunning={false} onClear={onClear} />);
    screen.getByText('Clear').click();
    expect(onClear).toHaveBeenCalled();
  });

  it('has view mode toggle buttons', () => {
    render(<StreamView events={[mockEvent('text', 'test')]} isRunning={false} />);
    expect(screen.getByText('Stream')).toBeInTheDocument();
    expect(screen.getByText('Logs')).toBeInTheDocument();
  });

  it('has auto-scroll checkbox', () => {
    render(<StreamView events={[mockEvent('text', 'test')]} isRunning={false} />);
    expect(screen.getByText('Auto-scroll')).toBeInTheDocument();
  });

  it('groups consecutive text events', () => {
    const events = [
      mockEvent('text', 'Line 1'),
      mockEvent('text', 'Line 2'),
      mockEvent('text', 'Line 3'),
    ];
    const { container } = render(<StreamView events={events} isRunning={false} />);
    // All lines should be combined into one pre element
    const preElements = container.querySelectorAll('pre');
    // Should have fewer pre elements than events due to grouping
    expect(preElements.length).toBeLessThan(events.length);
  });

  it('does not group output events', () => {
    const events = [
      mockEvent('output', 'Response 1'),
      mockEvent('output', 'Response 2'),
    ];
    render(<StreamView events={events} isRunning={false} />);
    // Both agent responses should be visible separately
    expect(screen.getByText('Response 1')).toBeInTheDocument();
    expect(screen.getByText('Response 2')).toBeInTheDocument();
  });

  it('shows tool call duration when available', () => {
    const events = [
      mockEvent('tool_result', 'Success', {
        tool_output: 'result',
        duration_ms: 150,
      }),
    ];
    render(<StreamView events={events} isRunning={false} />);
    expect(screen.getByText('150ms')).toBeInTheDocument();
  });

  it('truncates long tool input in collapsed state', () => {
    const longInput = 'a'.repeat(200);
    const events = [
      mockEvent('tool_call', '', {
        tool_name: 'test_tool',
        tool_input: longInput,
      }),
    ];
    render(<StreamView events={events} isRunning={false} />);
    // The preview should be truncated with ...
    const previewText = screen.getByText(/a{50,}\.{3}/);
    expect(previewText).toBeInTheDocument();
  });

  it('handles events with missing timestamps gracefully', () => {
    const events: StreamEvent[] = [
      {
        type: 'text',
        content: 'No timestamp',
        timestamp: '',
      },
    ];
    expect(() => render(<StreamView events={events} isRunning={false} />)).not.toThrow();
  });

  it('handles events with invalid JSON in tool_input', () => {
    const events = [
      mockEvent('tool_call', '', {
        tool_name: 'test_tool',
        tool_input: 'not valid json {',
      }),
    ];
    expect(() => render(<StreamView events={events} isRunning={false} />)).not.toThrow();
  });
});
