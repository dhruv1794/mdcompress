const RULES = [
  { name: 'strip-frontmatter', tier: 'safe', default: true },
  { name: 'strip-setext-headers', tier: 'safe', default: true },
  { name: 'strip-html-comments', tier: 'safe', default: true },
  { name: 'compress-code-blocks', tier: 'safe', default: true },
  { name: 'strip-badges', tier: 'safe', default: true },
  { name: 'strip-decorative-images', tier: 'safe', default: true },
  { name: 'strip-toc', tier: 'safe', default: true },
  { name: 'strip-trailing-cta', tier: 'safe', default: true },
  { name: 'strip-metadata-lines', tier: 'safe', default: true },
  { name: 'strip-marketing-prose', tier: 'aggressive', default: true },
  { name: 'strip-hedging-phrases', tier: 'aggressive', default: true },
  { name: 'strip-cross-references', tier: 'aggressive', default: true },
  { name: 'strip-admonition-prefixes', tier: 'aggressive', default: true },
  { name: 'strip-benchmark-prose', tier: 'aggressive', default: true },
  { name: 'dedup-cross-section', tier: 'aggressive', default: true },
  { name: 'strip-verification-boilerplate', tier: 'aggressive', default: true },
  { name: 'strip-boilerplate-sections', tier: 'aggressive', default: false },
  { name: 'collapse-example-output', tier: 'aggressive', default: false },
  { name: 'collapse-blank-lines', tier: 'safe', default: true },
];

export function ruleList() {
  return RULES;
}

export function compress(content, tier) {
  const tiers = { safe: 1, aggressive: 2, llm: 3 };
  const maxTier = tiers[tier] || 2;
  const fired = {};
  let output = content;
  let bytesBefore = new TextEncoder().encode(content).length;

  const applicable = RULES.filter(r => {
    const t = tiers[r.tier] || 1;
    return t <= maxTier && r.default;
  });

  for (const rule of applicable) {
    const result = applyRule(output, rule.name, maxTier);
    if (result.output !== output) {
      fired[rule.name] = result.nodes || 1;
      output = result.output;
    }
  }

  const bytesAfter = new TextEncoder().encode(output).length;
  const tokensBefore = Math.ceil(bytesBefore / 4);
  const tokensAfter = Math.ceil(bytesAfter / 4);

  return { output, tokensBefore, tokensAfter, bytesBefore, bytesAfter, rulesFired: fired };
}

function applyRule(text, name, maxTier) {
  switch (name) {
    case 'strip-frontmatter': return stripFrontmatter(text);
    case 'strip-setext-headers': return stripSetextHeaders(text);
    case 'strip-html-comments': return stripHTMLComments(text);
    case 'compress-code-blocks': return compressCodeBlocks(text, maxTier);
    case 'strip-badges': return stripBadges(text);
    case 'strip-decorative-images': return stripDecorativeImages(text);
    case 'strip-toc': return stripTOC(text);
    case 'strip-trailing-cta': return stripTrailingCTA(text);
    case 'strip-metadata-lines': return stripMetadataLines(text);
    case 'strip-marketing-prose': return stripMarketingProse(text);
    case 'strip-hedging-phrases': return stripHedgingPhrases(text);
    case 'strip-cross-references': return stripCrossReferences(text);
    case 'strip-admonition-prefixes': return stripAdmonitionPrefixes(text);
    case 'strip-benchmark-prose': return stripBenchmarkProse(text);
    case 'dedup-cross-section': return dedupCrossSection(text);
    case 'strip-verification-boilerplate': return stripVerificationBoilerplate(text);
    case 'strip-boilerplate-sections': return stripBoilerplateSections(text);
    case 'collapse-example-output': return collapseExampleOutput(text);
    case 'collapse-blank-lines': return collapseBlankLines(text);
    default: return { output: text, nodes: 0 };
  }
}

function stripFrontmatter(text) {
  if (/^---\r?\n/.test(text)) {
    const idx = text.indexOf('\n---', 3);
    if (idx > 0) {
      const end = text.indexOf('\n', idx + 4);
      return { output: end > 0 ? text.slice(end + 1) : text.slice(idx + 4), nodes: 1 };
    }
  }
  if (/^\+\+\+\r?\n/.test(text)) {
    const idx = text.indexOf('\n+++', 3);
    if (idx > 0) {
      const end = text.indexOf('\n', idx + 4);
      return { output: end > 0 ? text.slice(end + 1) : text.slice(idx + 4), nodes: 1 };
    }
  }
  return { output: text, nodes: 0 };
}

function stripSetextHeaders(text) {
  const lines = text.split('\n');
  let changed = false;
  let inFence = false;
  for (let i = 1; i < lines.length; i++) {
    const prev = lines[i - 1];
    const curr = lines[i];
    if (inFence && !/^```|^~~~/.test(curr.trim())) continue;
    if (/^```|^~~~/.test(curr.trim())) { inFence = !inFence; continue; }
    if (inFence) continue;
    if (!prev.trim()) continue;
    if (/^(#|\||>)/.test(prev.trim())) continue;
    const trimmedCurr = curr.trim();
    const isH1 = /^=+$/.test(trimmedCurr);
    const isH2 = /^-+$/.test(trimmedCurr);
    if (!isH1 && !isH2) continue;
    const prefix = isH1 ? '# ' : '## ';
    const headingText = prev.trim();
    lines[i - 1] = prefix + headingText;
    lines[i] = '';
    changed = true;
    i++;
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripHTMLComments(text) {
  const lines = text.split('\n');
  let changed = false;
  let inFence = false;
  for (let i = 0; i < lines.length; i++) {
    if (/^```|^~~~/.test(lines[i].trim())) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;
    const comment = lines[i].match(/<!--[\s\S]*?-->/);
    if (comment) {
      lines[i] = lines[i].replace(/<!--[\s\S]*?-->/g, '').trimEnd();
      if (!lines[i].trim()) {
        lines[i] = '';
      }
      changed = true;
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripBadges(text) {
  const badgeDomains = /(shields\.io|badge\.fury\.io|travis-ci\.(org|com)|circleci\.com|coveralls\.io|codecov\.io|camo\.githubusercontent|github\.com\/[^/]+\/[^/]+\/actions\/workflows|codacy\.com|snyk\.io|sonarcloud\.io|img\.shields)/i;
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    if (/^\[!\[/.test(lines[i].trim())) {
      const imgMatch = lines[i].match(/!\[[^\]]*\]\(([^)]+)\)/);
      if (imgMatch && badgeDomains.test(imgMatch[1])) {
        lines[i] = '';
        changed = true;
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripDecorativeImages(text) {
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (/^!\[/.test(trimmed)) {
      const altMatch = trimmed.match(/^!\[([^\]]*)\]/);
      if (altMatch) {
        const alt = altMatch[1].trim();
        if (alt === '' || /^(logo|banner|hero|header|divider|icon|screenshot|pic)$/i.test(alt)) {
          lines[i] = '';
          changed = true;
        }
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripTOC(text) {
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (/^#+\s*(Table of Contents|Contents|TOC)$/i.test(trimmed)) {
      lines[i] = '';
      let j = i + 1;
      while (j < lines.length && (/^\s*- \[/.test(lines[j]) || /^\s*$/.test(lines[j]))) {
        if (/^\s*- \[/.test(lines[j])) {
          lines[j] = '';
          changed = true;
        }
        j++;
      }
      changed = true;
      i = j - 1;
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripTrailingCTA(text) {
  const lines = text.split('\n');
  let changed = false;
  const total = text.length;
  for (let i = lines.length - 1; i >= 0 && i > lines.length * 0.7; i--) {
    const trimmed = lines[i].trim();
    if (/^#+\s*(Sponsors?|Backers?|Support|Connect|Social|Community|Star|Follow|Acknowledgements?|Credits|Thanks)\b/i.test(trimmed)) {
      lines[i] = '';
      changed = true;
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripMetadataLines(text) {
  const lines = text.split('\n');
  let changed = false;
  const pattern = /^(\*{0,2})(last\s*updated|updated|version|since|author|date|created|modified|published|status|repository|repo)\s*\*{0,2}\s*:/i;
  for (let i = 0; i < lines.length; i++) {
    if (pattern.test(lines[i].trim())) {
      lines[i] = '';
      changed = true;
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripMarketingProse(text) {
  const phrases = [
    /\bblazing(?:ly)? fast\b/gi,
    /\blightning fast\b/gi,
    /\bsuper-?fast\b/gi,
    /\bincredibly fast\b/gi,
    /\bproduction-ready\b/gi,
    /\bproduction-grade\b/gi,
    /\bbattle-tested\b/gi,
    /\bloved by developers\b/gi,
    /\bdeveloper-first\b/gi,
    /\bdeveloper-friendly\b/gi,
    /\bfeature-rich\b/gi,
    /\bfully-featured\b/gi,
    /\bcutting-edge\b/gi,
    /\bstate-of-the-art\b/gi,
    /\brock-solid\b/gi,
    /\bhighly performant\b/gi,
    /\bdep(?:endably|endable)\b/gi,
    /\bworld-class\b/gi,
    /\bindustry-leading\b/gi,
    /\benterprise-grade\b/gi,
    /\bbest-in-class\b/gi,
    /\bnext-generation\b/gi,
    /\bseamless(?:ly)?\b/gi,
    /\bintuitive\b/gi,
    /\bunparalleled\b/gi,
    /\bground-?breaking\b/gi,
    /\brevolutionary\b/gi,
  ];
  const lines = text.split('\n');
  let changed = false;
  let inFence = false;
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (/^```|^~~~/.test(trimmed)) { inFence = !inFence; continue; }
    if (inFence) continue;
    for (const phrase of phrases) {
      if (phrase.test(lines[i])) {
        lines[i] = lines[i].replace(phrase, '');
        changed = true;
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripHedgingPhrases(text) {
  const replacements = [
    [/\bit is worth noting that\s+/gi, ''],
    [/\bit is worth noting\s+/gi, ''],
    [/\bplease note that\s+/gi, ''],
    [/\bplease note\s+/gi, ''],
    [/\bit should be noted that\s+/gi, ''],
    [/\bit is important to note that\s+/gi, ''],
    [/\bneedless to say,?\s+/gi, ''],
    [/\bit goes without saying that\s+/gi, ''],
    [/\bas a matter of fact,?\s+/gi, ''],
    [/\bit should be mentioned that\s+/gi, ''],
    [/\bin order to\b/gi, 'to'],
    [/\bdue to the fact that\b/gi, 'because'],
    [/\bat this point in time\b/gi, 'now'],
    [/\bin the event that\b/gi, 'if'],
    [/\bwith regard to\b/gi, 'about'],
    [/\bfor the purpose of\b/gi, 'for'],
    [/\ba number of\b/gi, 'several'],
  ];
  let changed = false;
  let output = text;
  for (const [pattern, replacement] of replacements) {
    if (pattern.test(output)) {
      output = output.replace(pattern, replacement);
      changed = true;
    }
  }
  return changed ? { output, nodes: 1 } : { output: text, nodes: 0 };
}

function stripCrossReferences(text) {
  const patterns = [
    /(?:^|\n)\s*(?:See|For more (?:information|details)|Refer to|Check out|Read (?:more )?(?:about it )?in|Head (?:over|back) to|Go to|Navigate to|Visit|Continue reading (?:in|at)) (?:the )?\[[^\]]+\]\([^)]+\)(?: (?:section|guide|page|doc|document|chapter|tutorial|reference|manual))?\s*(?:for (?:more |further )?(?:information|details|guidelines?))?[.;]?\s*/gi,
    /(?:^|\n)\s*(?:More (?:details|information) (?:can be found|are available) (?:in|at) )\[[^\]]+\]\([^)]+\)[.;]?\s*/gi,
    /(?:^|\n)\s*See also:?\s*\[[^\]]+\]\([^)]+\)[.;]?\s*/gi,
  ];
  let changed = false;
  let output = text;
  for (const pattern of patterns) {
    if (pattern.test(output)) {
      output = output.replace(pattern, '\n');
      changed = true;
    }
  }
  return changed ? { output, nodes: 1 } : { output: text, nodes: 0 };
}

function stripAdmonitionPrefixes(text) {
  const pattern = /^(\s*>\s*)?(\*\*|__)?\s*(?:⚠\ufe0f?|💡|📖|⚡|🔍|💥|🔥|✅|❌|🚧|🚨|🎉|💬|📣|📢|🤖|🔧)?\s*(NOTE|WARNING|IMPORTANT|TIP|INFO|CAUTION|DANGER|HINT|REMINDER)\s*:?\s*(\*\*|__)?\s*/gim;
  let changed = false;
  let output = text.replace(pattern, (match, bp) => {
    changed = true;
    return bp ? bp + ' ' : '';
  });
  return changed ? { output, nodes: 1 } : { output: text, nodes: 0 };
}

function stripBenchmarkProse(text) {
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length - 2; i++) {
    const line = lines[i].trim();
    if (!line || /^```|^~~~/.test(line)) continue;
    if (/^#/.test(line) || /^\|/.test(line)) continue;
    const next = lines[i + 1].trim();
    if (/^\|.*\|/.test(next) && /^\|[-: |]+\|/.test(lines[i + 2]?.trim())) {
      const sentences = line.split(/[.!?]/).filter(s => s.trim());
      if (sentences.length <= 3) {
        lines[i] = '';
        changed = true;
        i += 2;
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function dedupCrossSection(text) {
  const paragraphs = text.split(/\n\n+/);
  if (paragraphs.length < 3) return { output: text, nodes: 0 };
  const intro = paragraphs.slice(0, 2);
  const body = paragraphs.slice(2);
  let changed = false;

  const introWords = intro.join(' ').toLowerCase().split(/\s+/).filter(w => w.length > 5);
  if (introWords.length < 5) return { output: text, nodes: 0 };

  for (let i = 0; i < body.length; i++) {
    const bodyWords = body[i].toLowerCase().split(/\s+/);
    const matchCount = introWords.filter(w => bodyWords.includes(w)).length;
    if (matchCount >= introWords.length * 0.7 && bodyWords.length > introWords.length) {
      body[i] = '';
      changed = true;
    }
  }

  return changed ? { output: paragraphs.join('\n\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripBoilerplateSections(text) {
  const headingMap = {
    'contributing': 'contributing\.md|welcome|please|read',
    'license': 'mit|apache|gpl|bsd|mpl|license\.md',
    'support|need help|questions|getting help': 'slack|discord|issue|discussion|forum|file a|reach out|join our',
    'code of conduct': 'code.of.conduct|covenant|enforcement',
  };
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (!/^#/.test(trimmed)) continue;
    const heading = trimmed.replace(/^#+\s*/, '').toLowerCase();
    for (const [key, pattern] of Object.entries(headingMap)) {
      const keyRegex = new RegExp('^' + key + '$', 'i');
      if (keyRegex.test(heading)) {
        const section = lines.slice(i, i + 10).join(' ').toLowerCase();
        const foundRegex = new RegExp(pattern, 'i');
        if (foundRegex.test(section)) {
          const level = trimmed.match(/^#+/)[0].length;
          let end = i + 1;
          while (end < lines.length) {
            const t = lines[end].trim();
            if (t && t.match(/^#+/)) {
              const nextLevel = t.match(/^#+/)[0].length;
              if (nextLevel <= level) break;
            }
            end++;
          }
          for (let k = i; k < end; k++) lines[k] = '';
          changed = true;
          i = end - 1;
        }
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function collapseExampleOutput(text) {
  const lines = text.split('\n');
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (!/^```|^~~~/.test(trimmed) || lines[i].trimStart() === trimmed) continue;
    const prevLine = i > 0 ? lines[i - 1].trim() : '';
    if (!/(--help|-h|--version|help|usage)/i.test(prevLine)) continue;
    const fenceMarker = trimmed[0];
    let j = i + 1;
    while (j < lines.length && !lines[j].trim().startsWith(fenceMarker.repeat(3))) j++;
    if (j > i + 1 && j - i - 1 <= 50) {
      const content = lines.slice(i, j + 1);
      const nonBlank = content.filter(l => l.trim() && !l.trim().startsWith(fenceMarker));
      const flags = nonBlank.filter(l => /^\s*-{1,2}[A-Za-z]/.test(l.trim()));
      if (flags.length >= nonBlank.length * 0.5) {
        for (let k = i; k <= j; k++) lines[k] = '';
        changed = true;
        i = j;
      }
    }
  }
  return changed ? { output: lines.join('\n'), nodes: 1 } : { output: text, nodes: 0 };
}

function stripVerificationBoilerplate(text) {
  const patterns = [
    /^\s*if (valid|successful),?\s+the output (is|will be|should be|looks like)\s*:?\s*$/gim,
    /^\s*if the (check|command|test) (fails|succeeds|passes),?\s+.*$/gim,
    /^\s*if (you do|it does) not (see|encounter|get) (an |a )?error,?\s+it means\s+.*$/gim,
    /^\s*if (no )?error(s)? occur(s)?,?\s+.*$/gim,
    /^\s*the (expected )?output (is|will be|should be|looks like)\s*:?\s*$/gim,
    /^\s*(you should see|you will see|you should get|you will get)\s+.*$/gim,
    /^\s*if (everything|all) (is|goes|went) (well|correctly),?\s+.*$/gim,
    /^\s*if (the )?above (command|step)s? (succeeds|executed? successfully|completed? successfully|ran? without errors?),?\s+.*$/gim,
  ];
  let changed = false;
  let output = text;
  for (const pattern of patterns) {
    if (pattern.test(output)) {
      output = output.replace(pattern, '\n');
      changed = true;
    }
  }
  return changed ? { output, nodes: 1 } : { output: text, nodes: 0 };
}

function collapseBlankLines(text) {
  const result = text.replace(/\n{3,}/g, '\n\n');
  const trimmedStart = result.replace(/^\n+/, '');
  if (result !== trimmedStart) return { output: trimmedStart, nodes: 1 };
  return result !== text ? { output: result, nodes: 1 } : { output: text, nodes: 0 };
}

function compressCodeBlocks(text, maxTier) {
  const lines = text.split('\n');
  const blocks = [];
  let inFence = false, fenceMarker = '', fenceLang = '', fenceLine = -1;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    const m = t.match(/^(`{3,}|~{3,})/);
    if (m) {
      if (!inFence) {
        inFence = true;
        fenceMarker = m[1][0];
        fenceLang = t.slice(m[1].length).trim().toLowerCase().split(/\s+/)[0] || '';
        fenceLine = i;
      } else if (m[1][0] === fenceMarker) {
        inFence = false;
        if (fenceLine >= 0 && i > fenceLine + 1) {
          blocks.push({ start: fenceLine, end: i, lang: fenceLang, content: lines.slice(fenceLine + 1, i) });
        }
        fenceLine = -1;
      }
    }
  }

  if (blocks.length === 0) return { output: text, nodes: 0 };

  let changed = false;
  const seenHashes = new Set();

  for (let bi = 0; bi < blocks.length; bi++) {
    const block = blocks[bi];
    const content = [...block.content];
    const origHash = hashLines(content);

    if (maxTier >= 2 && seenHashes.has(origHash)) {
      block.content = ['[duplicate — same as block above]'];
      changed = true;
      continue;
    }

    let mod = false;
    mod = stripShellPromptsJS(content, block.lang) || mod;
    mod = stripConfigCommentsJS(content, block.lang) || mod;

    if (maxTier >= 2) {
      mod = stripImportsJS(content, block.lang) || mod;
      mod = stripErrorBoilerplateJS(content, block.lang) || mod;
    }

    if (mod) {
      const filtered = content.filter(l => l.trim() !== '');
      block.content = filtered;
      changed = true;
    }

    if (maxTier >= 2) seenHashes.add(origHash);
  }

  if (maxTier >= 2 && blocks.length > 1) {
    for (let i = 1; i < blocks.length; i++) {
      const p = blocks[i - 1], c = blocks[i];
      if (p.lang && c.lang && p.lang !== c.lang && !c.content[0]?.startsWith('[')) {
        const pn = p.content.join('\n').replace(/\s/g, '').toLowerCase();
        const cn = c.content.join('\n').replace(/\s/g, '').toLowerCase();
        if (pn === cn) {
          c.content = [`[identical to ${p.lang} example above]`];
          changed = true;
        }
      }
    }
  }

  if (!changed) return { output: text, nodes: 0 };

  const out = [];
  let blockIdx = 0;
  for (let i = 0; i < lines.length; i++) {
    const block = blocks[blockIdx];
    if (block && i === block.start) {
      out.push(lines[i]);
      out.push(...block.content);
      out.push(lines[block.end]);
      i = block.end;
      blockIdx++;
    } else {
      out.push(lines[i]);
    }
  }

  return { output: out.join('\n'), nodes: blocks.length };
}

function hashLines(lines) {
  let h = 0;
  for (const l of lines) {
    for (let i = 0; i < l.length; i++) { h = ((h << 5) - h) + l.charCodeAt(i); h |= 0; }
  }
  return h;
}

function stripShellPromptsJS(lines, lang) {
  const isShell = ['sh', 'bash', 'zsh', 'fish', 'ksh', 'shell', 'console', 'terminal', 'powershell', 'pwsh'].includes(lang);
  if (!isShell && !hasShellPromptsJS(lines)) return false;
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const m = lines[i].match(/^(\$\s*|>\s*|#\s+)(.*)/);
    if (m) { lines[i] = m[2]; changed = true; continue; }
    if (isShell && (/^#!\//.test(lines[i]) || /^set\s+[-+][a-zA-Z]/.test(lines[i]))) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function hasShellPromptsJS(lines) {
  let c = 0;
  for (let i = 0; i < lines.length && i < 10; i++) { if (/^\$\s/.test(lines[i])) c++; }
  return c >= 2;
}

function stripConfigCommentsJS(lines, lang) {
  const isConfig = ['yaml', 'yml', 'toml', 'ini', 'cfg', 'conf', 'properties', 'env', 'editorconfig', 'gitconfig'].includes(lang);
  if (!isConfig) return false;
  let changed = false;
  const isINI = ['ini', 'cfg', 'conf', 'properties', 'editorconfig', 'gitconfig'].includes(lang);
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (!t) continue;
    if (isINI ? /^\s*[#;]/.test(lines[i]) : /^\s*#/.test(lines[i])) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripImportsJS(lines, lang) {
  switch (lang) {
    case 'golang': case 'go': return stripGoImportsJS(lines);
    case 'python': case 'py': case 'py3': return stripPythonImportsJS(lines);
    case 'javascript': case 'js': case 'typescript': case 'ts': case 'jsx': case 'tsx': case 'mjs': case 'cjs': return stripJSImportsJS(lines);
    case 'java': case 'scala': case 'kotlin': return stripJavaImportsJS(lines);
    case 'c': case 'cpp': case 'c++': case 'h': case 'hpp': case 'cxx': case 'cc': return stripCImportsJS(lines);
    case 'rust': case 'rs': return stripRustImportsJS(lines);
    default: return false;
  }
}

function stripGoImportsJS(lines) {
  let changed = false, inParen = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (inParen) { lines[i] = ''; changed = true; if (/^\)$/.test(t)) inParen = false; continue; }
    if (/^import\s*\($/.test(t)) { lines[i] = ''; inParen = true; changed = true; continue; }
    if (/^import\s+"[^"]*"$/.test(t)) { lines[i] = ''; changed = true; continue; }
    if (t === 'package main' || t.startsWith('package ')) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripPythonImportsJS(lines) {
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (/^(import\s+\S|from\s+\S+\s+import)/.test(t) || /^if\s+__name__\s*==\s*['"]__main__['"]\s*:/.test(t) || /^#!\/usr\/bin\/(env\s+)?python/.test(t)) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripJSImportsJS(lines) {
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (/^(import\s+|export\s+)/.test(t) || /^(const|var|let)\s+\w+\s*=\s*require\(/.test(t) || /^module\.exports\s*=/.test(t) || /^import\s+type\s+/.test(t)) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripJavaImportsJS(lines) {
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    if (/^(import\s+|package\s+)\S/.test(lines[i].trim())) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripCImportsJS(lines) {
  let changed = false, inIfndef = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (/^#include\s*[<"]/.test(t) || /^using\s+(namespace|static)\s+/.test(t) || /^#pragma\s/.test(t)) { lines[i] = ''; changed = true; continue; }
    if (/^#ifndef\s/.test(t)) { inIfndef = true; lines[i] = ''; changed = true; continue; }
    if (inIfndef) { if (!t || /^#define\s/.test(t)) { lines[i] = ''; changed = true; continue; } if (/^#endif/.test(t)) { lines[i] = ''; changed = true; inIfndef = false; } }
  }
  return changed;
}

function stripRustImportsJS(lines) {
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    const t = lines[i].trim();
    if (/^use\s+\S/.test(t) || /^extern\s+crate\s+/.test(t) || /^mod\s+\S/.test(t) || /^#\[derive\(/.test(t)) { lines[i] = ''; changed = true; }
  }
  return changed;
}

function stripErrorBoilerplateJS(lines, lang) {
  if (lang !== 'go' && lang !== 'golang') return false;
  let changed = false;
  for (let i = 0; i < lines.length; i++) {
    if (!/^\s*if\s+err\s*!=\s*nil\s*\{?\s*$/.test(lines[i].trim())) continue;
    changed = true;
    lines[i] = '';
    if (i + 1 < lines.length && /^\s*return\s+(nil,\s*)?\w*err\w*\s*$/.test(lines[i + 1].trim())) { lines[i + 1] = ''; i++; }
  }
  return changed;
}
