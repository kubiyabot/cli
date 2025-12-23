import * as esbuild from 'esbuild';
import * as fs from 'fs';
import * as path from 'path';

const isProd = process.argv.includes('--prod');
const isWatch = process.argv.includes('--watch');

// Output directory
const outDir = '../internal/webui/static';

// Ensure output directory exists
if (!fs.existsSync(outDir)) {
  fs.mkdirSync(outDir, { recursive: true });
}

// Build configuration
const buildOptions = {
  entryPoints: ['src/index.tsx'],
  bundle: true,
  minify: isProd,
  sourcemap: !isProd,
  outfile: path.join(outDir, 'bundle.js'),
  format: 'esm',
  target: ['es2020'],
  jsxFactory: 'h',
  jsxFragment: 'Fragment',
  define: {
    'process.env.NODE_ENV': isProd ? '"production"' : '"development"',
  },
  loader: {
    '.tsx': 'tsx',
    '.ts': 'ts',
  },
};

// Copy index.html to output
const indexHtml = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Kubiya Worker Pool</title>
  <link rel="stylesheet" href="styles.css">
</head>
<body>
  <div id="app"></div>
  <script type="module" src="bundle.js"></script>
</body>
</html>`;

fs.writeFileSync(path.join(outDir, 'index.html'), indexHtml);

// Copy CSS
const cssContent = fs.readFileSync('src/styles/index.css', 'utf-8');
fs.writeFileSync(path.join(outDir, 'styles.css'), cssContent);

async function build() {
  try {
    if (isWatch) {
      const ctx = await esbuild.context(buildOptions);
      await ctx.watch();
      console.log('Watching for changes...');
    } else {
      await esbuild.build(buildOptions);
      console.log('Build complete');

      // Report bundle size
      const stats = fs.statSync(path.join(outDir, 'bundle.js'));
      const sizeKB = (stats.size / 1024).toFixed(2);
      console.log(`Bundle size: ${sizeKB} KB`);
    }
  } catch (err) {
    console.error('Build failed:', err);
    process.exit(1);
  }
}

build();
