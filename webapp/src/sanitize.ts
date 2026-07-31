// sanitizeHtml strips the dangerous parts of GLPI rich-text (HTML) content
// before it is rendered with dangerouslySetInnerHTML: <script> blocks,
// event-handler attributes, and javascript: URLs. It is a pragmatic filter for
// content produced by the GLPI knowledge base; callers should prefer text
// rendering wherever a sanitized HTML view is not required.
export function sanitizeHtml(html: string): string {
  if (!html) return '';
  let out = html.replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '');
  out = out.replace(/\son\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, '');
  out = out.replace(/\shref\s*=\s*("'?)javascript:[^"'>]*\1/gi, '');
  return out;
}
