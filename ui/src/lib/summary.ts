// Records are often authored "CATEGORY (…": the badge already shows the
// category, so strip a leading ALLCAPS token equal to the category plus an
// optional "(" and surrounding punctuation/space.
export function stripCategoryPrefix(content: string, category: string): string {
  const re = new RegExp('^' + category.toUpperCase() + '\\s*\\(?\\s*[:\\-]?\\s*', '');
  return content.replace(re, '').trimStart();
}
