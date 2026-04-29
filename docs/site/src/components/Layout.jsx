import { NavLink } from 'react-router-dom';

export default function Layout({ children }) {
  return (
    <div className="wrap">
      <div className="hero">
        <h1>mdcompress</h1>
        <p className="sub">
          Markdown token reduction for agent context &mdash; deterministic, offline, safe.
        </p>
        <nav className="nav">
          <NavLink to="/" end>Benchmarks</NavLink>
          <NavLink to="/test">Test</NavLink>
          <a href="https://github.com/dhruv1794/mdcompress" target="_blank" rel="noopener">GitHub</a>
        </nav>
      </div>
      {children}
      <p className="footer">
        Updated weekly via CI. Token counts use <code>cl100k_base</code>; real Claude/GPT savings may differ &plusmn;10%.
        At Sonnet <a href="https://www.anthropic.com/pricing#anthropic-api">$3/MTok input</a>, every 1M tokens saved = $3.
        <a href="https://github.com/dhruv1794/mdcompress"> GitHub</a>
      </p>
    </div>
  );
}
