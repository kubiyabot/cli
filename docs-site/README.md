# Kubiya CLI Documentation Site

This directory contains the GitHub Pages documentation site for the Kubiya CLI.

## Structure

```
docs-site/
├── _config.yml              # Jekyll configuration
├── _layouts/                 # Page layouts
├── _includes/                # Reusable components
├── assets/                   # CSS, JS, images
├── pages/                    # Documentation pages
├── examples/                 # Example files
├── index.md                  # Homepage
├── Gemfile                   # Ruby dependencies
└── .github/workflows/        # GitHub Actions for deployment
```

## Local Development

### Prerequisites

- Ruby 3.1 or higher
- Bundler gem

### Setup

1. Install dependencies:
```bash
cd docs-site
bundle install
```

2. Run development server:
```bash
bundle exec jekyll serve --livereload
```

3. Open http://localhost:4000 in your browser

### Building

To build the site for production:

```bash
bundle exec jekyll build
```

The built site will be in the `_site` directory.

## Deployment

The site is automatically deployed to GitHub Pages via GitHub Actions when changes are pushed to the `main` branch in the `docs-site/` directory.

## Contributing

### Adding New Pages

1. Create a new markdown file in the `pages/` directory
2. Add YAML front matter:
```yaml
---
layout: page
title: Page Title
description: Page description
toc: true  # Enable table of contents
---
```

3. Add the page to the navigation in `_config.yml`:
```yaml
navigation:
  - title: New Page
    url: /pages/new-page
```

### Adding Examples

1. Create markdown files in the `examples/` directory
2. Use the `example` layout:
```yaml
---
layout: example
title: Example Title
description: Example description
difficulty: beginner  # beginner, intermediate, advanced
category: workflow    # workflow, agent, tool, etc.
tags: [kubernetes, deployment]
---
```

### Styling

- Main styles are in `assets/css/main.css`
- JavaScript functionality is in `assets/js/main.js`
- The site uses a custom theme based on Minima

### Best Practices

1. **Use semantic HTML** in layouts and includes
2. **Include descriptive alt text** for images
3. **Test responsive design** on multiple screen sizes
4. **Validate links** before committing
5. **Use consistent formatting** for code examples
6. **Include table of contents** for long pages (`toc: true`)

## Features

- **Responsive design** that works on desktop and mobile
- **Syntax highlighting** for code blocks
- **Copy-to-clipboard** functionality for code examples
- **Search functionality** (if implemented)
- **Table of contents** generation
- **Interactive examples** with live demos
- **Dark/light mode** support (if implemented)

## Customization

### Colors

The site uses CSS custom properties for theming:

```css
:root {
  --primary-color: #667eea;
  --secondary-color: #764ba2;
  --accent-color: #ffd700;
  --text-color: #333;
  --bg-color: #ffffff;
}
```

### Fonts

The site uses system fonts for better performance:

```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
```

### Layout

The site uses CSS Grid and Flexbox for responsive layouts:

- Container max-width: 1200px
- Breakpoints: 768px (tablet), 1024px (desktop)
- Grid system for feature cards and examples

## Performance

- **Optimized images** with appropriate formats and sizes
- **Minified CSS and JS** in production
- **Lazy loading** for images (if implemented)
- **CDN delivery** for external resources
- **Caching headers** for static assets

## SEO

- **Meta tags** for description and keywords
- **Open Graph** tags for social sharing
- **Structured data** for rich snippets
- **XML sitemap** generated automatically
- **Robots.txt** for search engine crawling

## Accessibility

- **Semantic HTML** structure
- **Alt text** for images
- **Keyboard navigation** support
- **Screen reader** compatibility
- **High contrast** color scheme
- **Focus indicators** for interactive elements

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)
- Mobile browsers

## License

This documentation site is part of the Kubiya CLI project and follows the same license terms.