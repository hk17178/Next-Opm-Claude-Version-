import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AssetGradeTag } from './AssetGradeTag';
import type { AssetGrade } from '../types';

describe('AssetGradeTag', () => {
  it('renders the grade text', () => {
    render(<AssetGradeTag grade="S" />);
    expect(screen.getByText('S')).toBeInTheDocument();
  });

  it('renders all grade levels without error', () => {
    const grades: AssetGrade[] = ['S', 'A', 'B', 'C', 'D'];
    grades.forEach((grade) => {
      const { unmount } = render(<AssetGradeTag grade={grade} />);
      expect(screen.getByText(grade)).toBeInTheDocument();
      unmount();
    });
  });

  it('renders as an Ant Design Tag element', () => {
    const { container } = render(<AssetGradeTag grade="A" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag).toBeTruthy();
  });

  it('applies fontWeight 600 style', () => {
    const { container } = render(<AssetGradeTag grade="B" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag?.style.fontWeight).toBe('600');
  });

  it('applies border-radius 4px style', () => {
    const { container } = render(<AssetGradeTag grade="C" />);
    const tag = container.querySelector('.ant-tag');
    expect(tag?.style.borderRadius).toBe('4px');
  });
});
