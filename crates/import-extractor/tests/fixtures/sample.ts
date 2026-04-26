// Realistic TypeScript file exercising the import forms the extractor handles.

import React, { useState, useEffect } from 'react';
import type { FC, ReactNode } from 'react';
import { Button } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import debounce from 'lodash/debounce';
import path from 'node:path';

import { logger } from 'myorg/frontend/common';
import type { User, Session } from 'myorg/frontend/types';

import './styles.css';
import 'reflect-metadata';

export { Button as MyButton } from '@mui/material';
export * from './utils';
export type { Theme } from 'myorg/frontend/themes';

const Plugin = {
  postcssPlugin: 'demo',
  Once(root: import('postcss').Root) {
    root.walkRules((_rule: import('postcss').Rule) => {});
  },
};

export const App: FC<{ children: ReactNode }> = ({ children }) => {
  const [_, setX] = useState(0);
  useEffect(() => {
    const onResize = debounce(() => setX(window.innerWidth), 100);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  void useQuery({ queryKey: ['x'], queryFn: () => fetch('/api') });

  // Lazy-loaded component.
  const Lazy = async () => (await import('./LazyChild')).default;
  void Lazy;

  return <div data-base={path.basename('/x')}>{children}</div>;
};
