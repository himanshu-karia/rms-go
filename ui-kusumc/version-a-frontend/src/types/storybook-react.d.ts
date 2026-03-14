declare module '@storybook/react' {
  import type { ComponentType } from 'react';

  export type Meta<TArgs = Record<string, unknown>> = {
    title: string;
    component?: ComponentType<unknown>;
    args?: Partial<TArgs>;
    parameters?: Record<string, unknown>;
  };

  export type StoryObj<TArgs = Record<string, unknown>> = {
    args?: Partial<TArgs>;
    render?: (args: TArgs) => JSX.Element;
    parameters?: Record<string, unknown>;
  };
}
