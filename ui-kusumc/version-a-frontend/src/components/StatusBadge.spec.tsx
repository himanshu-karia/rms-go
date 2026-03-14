import { render, screen } from '@testing-library/react';

import { StatusBadge } from './StatusBadge';

describe('StatusBadge', () => {
  it('renders normalized status text when no label provided', () => {
    render(<StatusBadge status="ONLINE" />);

    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('renders custom label when provided', () => {
    render(<StatusBadge status="offline" label="Disconnected" />);

    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });
});
