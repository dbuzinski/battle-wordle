import { describe, it, expect } from 'vitest';
import { getHardModeError } from './hardMode';

describe('getHardModeError', () => {
  it('blocks yellow letter in same position (example: EARTH -> CARTE)', () => {
    // EARTH vs REACT: E(p), A(p), R(p), T(a), H(a)
    // A, R, T are present in positions 2,3,4
    // Next guess CARTE: A in pos2, R in pos3, T in pos4 (should be blocked)
    expect(getHardModeError('CARTE', ['EARTH'], [['present','present','present','absent','absent']])).toMatch(/cannot be in position/);
  });

  it('allows valid yellow placement', () => {
    // EARTH vs REACT: E(p), A(p), R(p), T(a), H(a)
    // Next guess: CRATE (A in pos3, R in pos2, T in pos4)
    expect(getHardModeError('CRATE', ['EARTH'], [['present','present','present','absent','absent']])).toBe(null);
  });

  it('blocks missing yellow letter', () => {
    // EARTH vs REACT: E(p), A(p), R(p), T(a), H(a)
    // Next guess: CHESS (no A, R, or T)
    expect(getHardModeError('CHESS', ['EARTH'], [['present','present','present','absent','absent']])).toMatch(/Must use/);
  });

  it('blocks missing green letter in position', () => {
    // REACT vs REACT: all correct
    // Next guess: RECAP (should block P in pos5)
    expect(getHardModeError('RECAP', ['REACT'], [['correct','correct','correct','correct','correct']])).toMatch(/Must use T in position 5/);
  });

  it('blocks gray letter reuse', () => {
    // EARTH vs REACT: T(a), H(a) -> T, H forbidden
    expect(getHardModeError('THREE', ['EARTH'], [['present','present','present','absent','absent']])).toMatch(/Cannot use T/);
  });

  it('enforces double letter present (FENCE -> JEEPS)', () => {
    // FENCE vs REACT: F(a), E(c), N(a), C(a), E(p)
    // E is correct in pos2, present in pos4
    // Next guess: JEEPS (E in pos2, E in pos3, S in pos5)
    // Should allow (E in pos2, E not in pos4)
    expect(getHardModeError('JEEPS', ['FENCE'], [['absent','correct','absent','present','absent']])).toBe(null);
  });

  it('blocks double letter present in forbidden position (FENCE -> TERSE)', () => {
    // FENCE vs REACT: E(c), N(a), C(a), E(p), F(a)
    // E is correct in pos2, present in pos4
    // Next guess: TERSE (E in pos2, E in pos5)
    // Should block E in pos5 (forbidden)
    expect(getHardModeError('TERSE', ['FENCE'], [['absent','correct','absent','present','absent']])).toMatch(/E cannot be in position 5/);
  });
}); 