export type FeedbackType = 'correct' | 'present' | 'absent';

const WORD_LENGTH = 5;

type PresentInfo = { min: number; forbiddenPos: Set<number> };

export function getHardModeError(
  guess: string,
  guesses: string[],
  feedback: FeedbackType[][]
): string | null {
  if (guesses.length === 0) return null;
  const required: (string | null)[] = Array(WORD_LENGTH).fill(null); // green
  const forbidden: Set<string> = new Set(); // gray
  const present: { [letter: string]: PresentInfo } = {};
  for (let g = 0; g < guesses.length; g++) {
    const prev = guesses[g];
    const fb = feedback[g];
    for (let i = 0; i < WORD_LENGTH; i++) {
      const l = prev[i];
      if (fb[i] === 'correct') {
        required[i] = l;
      } else if (fb[i] === 'absent') {
        forbidden.add(l);
      } else if (fb[i] === 'present') {
        if (!present[l]) present[l] = { min: 0, forbiddenPos: new Set() };
        present[l].min += 1;
        present[l].forbiddenPos.add(i);
      }
    }
  }
  for (let i = 0; i < WORD_LENGTH; i++) {
    if (required[i] && guess[i] !== required[i]) {
      return `Hard mode: Must use ${required[i]} in position ${i + 1}`;
    }
  }
  for (let i = 0; i < WORD_LENGTH; i++) {
    if (forbidden.has(guess[i])) {
      return `Hard mode: Cannot use ${guess[i]} (marked absent)`;
    }
  }
  for (const l in present) {
    const count = guess.split('').filter((c) => c === l).length;
    if (count < present[l].min) {
      return `Hard mode: Must use ${l} at least ${present[l].min} time(s)`;
    }
    for (const pos of present[l].forbiddenPos) {
      if (guess[pos] === l) {
        return `Hard mode: ${l} cannot be in position ${pos + 1}`;
      }
    }
  }
  return null;
} 