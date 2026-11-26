import { SNIPPETS, suggestions } from './snippets';

describe('snippets', () => {
  it('offers every snippet as an editor suggestion', () => {
    const items = suggestions();
    expect(items).toHaveLength(SNIPPETS.length);
    for (const item of items) {
      expect(item.insertText).toMatch(/\{\{.*\}\}/s);
    }
  });

  it('balances block snippets', () => {
    for (const s of SNIPPETS) {
      const opens = (s.insert.match(/\{\{ (range|if|with) /g) ?? []).length;
      const ends = (s.insert.match(/\{\{ end \}\}/g) ?? []).length;
      expect(ends).toBe(opens);
    }
  });
});
