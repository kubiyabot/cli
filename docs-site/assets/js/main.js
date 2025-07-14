/**
 * Modern Documentation Site JavaScript
 * Handles dark mode, navigation, and interactive features
 */

(function() {
  'use strict';

  // Initialize when DOM is ready
  document.addEventListener('DOMContentLoaded', function() {
    initializeDarkMode();
    initializeNavigation();
    initializeSidebar();
    initializeTableOfContents();
    initializeInstallTabs();
    initializeCodeCopy();
    initializeAnimations();
    removeLineNumbers();
  });

  /**
   * Dark Mode Functionality
   */
  function initializeDarkMode() {
    const darkModeToggle = document.getElementById('dark-mode-toggle');
    const html = document.documentElement;
    
    if (!darkModeToggle) return;

    // Check for saved theme preference or default to 'light'
    const savedTheme = localStorage.getItem('theme');
    const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
    const defaultTheme = savedTheme || (systemPrefersDark ? 'dark' : 'light');
    
    // Apply the theme
    setTheme(defaultTheme);
    
    // Toggle theme on button click
    darkModeToggle.addEventListener('click', function() {
      const currentTheme = html.getAttribute('data-theme');
      const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
      setTheme(newTheme);
    });
    
    // Listen for system theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function(e) {
      if (!localStorage.getItem('theme')) {
        setTheme(e.matches ? 'dark' : 'light');
      }
    });
    
    function setTheme(theme) {
      html.setAttribute('data-theme', theme);
      localStorage.setItem('theme', theme);
      
      // Update toggle button icon
      const icon = darkModeToggle.querySelector('i');
      if (icon) {
        icon.className = theme === 'dark' ? 'fas fa-sun' : 'fas fa-moon';
      }
    }
  }

  /**
   * Navigation Functionality
   */
  function initializeNavigation() {
    const navToggle = document.getElementById('navbar-toggle');
    const navMenu = document.getElementById('navbar-nav');
    
    if (!navToggle || !navMenu) return;
    
    navToggle.addEventListener('click', function() {
      navMenu.classList.toggle('active');
      navToggle.classList.toggle('active');
    });
    
    // Close menu when clicking outside
    document.addEventListener('click', function(e) {
      if (!navToggle.contains(e.target) && !navMenu.contains(e.target)) {
        navMenu.classList.remove('active');
        navToggle.classList.remove('active');
      }
    });
    
    // Close menu when clicking on a link
    navMenu.querySelectorAll('a').forEach(link => {
      link.addEventListener('click', function() {
        navMenu.classList.remove('active');
        navToggle.classList.remove('active');
      });
    });
  }

  /**
   * Sidebar Functionality
   */
  function initializeSidebar() {
    const sidebar = document.getElementById('sidebar');
    const sidebarOverlay = document.getElementById('sidebar-overlay');
    const mobileSidebarToggle = document.getElementById('mobile-sidebar-toggle');
    const sidebarToggle = document.getElementById('sidebar-toggle');
    
    if (!sidebar) return;
    
    // Mobile sidebar toggle
    if (mobileSidebarToggle) {
      mobileSidebarToggle.addEventListener('click', function() {
        sidebar.classList.add('active');
        sidebarOverlay.classList.add('active');
        document.body.style.overflow = 'hidden';
      });
    }
    
    // Close sidebar
    function closeSidebar() {
      sidebar.classList.remove('active');
      sidebarOverlay.classList.remove('active');
      document.body.style.overflow = '';
    }
    
    if (sidebarToggle) {
      sidebarToggle.addEventListener('click', closeSidebar);
    }
    
    if (sidebarOverlay) {
      sidebarOverlay.addEventListener('click', closeSidebar);
    }
    
    // Close sidebar on escape key
    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape' && sidebar.classList.contains('active')) {
        closeSidebar();
      }
    });
  }

  /**
   * Table of Contents Generation
   */
  function initializeTableOfContents() {
    const tocNav = document.getElementById('toc-nav');
    const contentWrapper = document.querySelector('.content-wrapper');
    
    if (!tocNav || !contentWrapper) return;
    
    // Find all headings in the content
    const headings = contentWrapper.querySelectorAll('h1, h2, h3, h4, h5, h6');
    
    if (headings.length === 0) return;
    
    // Create table of contents
    const tocList = document.createElement('ul');
    tocList.className = 'toc-list';
    
    headings.forEach((heading, index) => {
      // Create unique ID for the heading
      const id = `heading-${index}`;
      heading.id = id;
      
      // Create TOC item
      const tocItem = document.createElement('li');
      tocItem.className = `toc-item toc-${heading.tagName.toLowerCase()}`;
      
      const tocLink = document.createElement('a');
      tocLink.href = `#${id}`;
      tocLink.textContent = heading.textContent;
      tocLink.className = 'toc-link';
      
      tocItem.appendChild(tocLink);
      tocList.appendChild(tocItem);
    });
    
    tocNav.appendChild(tocList);
    
    // Add scroll spy functionality
    initializeScrollSpy();
  }

  /**
   * Scroll Spy for Table of Contents
   */
  function initializeScrollSpy() {
    const tocLinks = document.querySelectorAll('.toc-link');
    const headings = document.querySelectorAll('.content-wrapper h1, .content-wrapper h2, .content-wrapper h3, .content-wrapper h4, .content-wrapper h5, .content-wrapper h6');
    
    if (tocLinks.length === 0 || headings.length === 0) return;
    
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach(entry => {
          const id = entry.target.id;
          const tocLink = document.querySelector(`.toc-link[href="#${id}"]`);
          
          if (entry.isIntersecting) {
            // Remove active class from all links
            tocLinks.forEach(link => link.classList.remove('active'));
            // Add active class to current link
            if (tocLink) {
              tocLink.classList.add('active');
            }
          }
        });
      },
      {
        rootMargin: '-100px 0px -80% 0px',
        threshold: 0
      }
    );
    
    headings.forEach(heading => {
      observer.observe(heading);
    });
  }

  /**
   * Installation Tabs Functionality
   */
  function initializeInstallTabs() {
    const tabs = document.querySelectorAll('.install-tab');
    const contents = document.querySelectorAll('.install-content');
    
    if (tabs.length === 0 || contents.length === 0) return;
    
    tabs.forEach(tab => {
      tab.addEventListener('click', function() {
        const target = this.dataset.tab;
        
        // Remove active class from all tabs and contents
        tabs.forEach(t => t.classList.remove('active'));
        contents.forEach(c => c.classList.remove('active'));
        
        // Add active class to clicked tab and corresponding content
        this.classList.add('active');
        const targetContent = document.getElementById(target);
        if (targetContent) {
          targetContent.classList.add('active');
        }
      });
    });
  }

  /**
   * Code Copy Functionality
   */
  function initializeCodeCopy() {
    const codeBlocks = document.querySelectorAll('pre');
    
    codeBlocks.forEach(block => {
      // Skip if already has copy button
      if (block.querySelector('.copy-button')) return;
      
      const copyButton = document.createElement('button');
      copyButton.className = 'copy-button';
      copyButton.innerHTML = '<i class="fas fa-copy"></i>';
      copyButton.title = 'Copy code';
      copyButton.setAttribute('aria-label', 'Copy code to clipboard');
      
      block.appendChild(copyButton);
      
      copyButton.addEventListener('click', async function() {
        const code = block.querySelector('code');
        if (!code) return;
        
        const text = code.textContent;
        
        try {
          await navigator.clipboard.writeText(text);
          
          // Update button to show success
          copyButton.innerHTML = '<i class="fas fa-check"></i>';
          copyButton.style.background = 'var(--color-success)';
          copyButton.title = 'Copied!';
          
          setTimeout(() => {
            copyButton.innerHTML = '<i class="fas fa-copy"></i>';
            copyButton.style.background = 'var(--color-primary)';
            copyButton.title = 'Copy code';
          }, 2000);
          
        } catch (err) {
          console.error('Failed to copy code:', err);
          
          // Fallback for older browsers
          const textArea = document.createElement('textarea');
          textArea.value = text;
          textArea.style.position = 'fixed';
          textArea.style.left = '-999999px';
          textArea.style.top = '-999999px';
          document.body.appendChild(textArea);
          textArea.focus();
          textArea.select();
          
          try {
            document.execCommand('copy');
            copyButton.innerHTML = '<i class="fas fa-check"></i>';
            copyButton.style.background = 'var(--color-success)';
            
            setTimeout(() => {
              copyButton.innerHTML = '<i class="fas fa-copy"></i>';
              copyButton.style.background = 'var(--color-primary)';
            }, 2000);
          } catch (err2) {
            console.error('Fallback copy failed:', err2);
          }
          
          document.body.removeChild(textArea);
        }
      });
    });
  }

  /**
   * Scroll Animations
   */
  function initializeAnimations() {
    // Intersection Observer for fade-in animations
    const observerOptions = {
      threshold: 0.1,
      rootMargin: '0px 0px -50px 0px'
    };
    
    const observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          entry.target.classList.add('animate-fade-in-up');
          observer.unobserve(entry.target);
        }
      });
    }, observerOptions);
    
    // Observe feature cards and example cards
    document.querySelectorAll('.feature-card, .example-card').forEach(card => {
      observer.observe(card);
    });
    
    // Smooth scrolling for anchor links
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
      anchor.addEventListener('click', function(e) {
        e.preventDefault();
        
        const target = document.querySelector(this.getAttribute('href'));
        if (target) {
          target.scrollIntoView({
            behavior: 'smooth',
            block: 'start'
          });
        }
      });
    });
  }

  /**
   * Search Functionality (if search input exists)
   */
  function initializeSearch() {
    const searchInput = document.getElementById('search-input');
    if (!searchInput) return;
    
    searchInput.addEventListener('input', function() {
      const query = this.value.toLowerCase();
      const searchableItems = document.querySelectorAll('.searchable-item');
      
      searchableItems.forEach(item => {
        const text = item.textContent.toLowerCase();
        const isMatch = text.includes(query);
        item.style.display = isMatch ? 'block' : 'none';
      });
    });
  }

  // Initialize search if needed
  document.addEventListener('DOMContentLoaded', initializeSearch);

  /**
   * Keyboard Navigation
   */
  document.addEventListener('keydown', function(e) {
    // Toggle dark mode with Ctrl/Cmd + D
    if ((e.ctrlKey || e.metaKey) && e.key === 'd') {
      e.preventDefault();
      const darkModeToggle = document.getElementById('dark-mode-toggle');
      if (darkModeToggle) {
        darkModeToggle.click();
      }
    }
    
    // Close mobile menu with Escape
    if (e.key === 'Escape') {
      const navMenu = document.getElementById('navbar-nav');
      const navToggle = document.getElementById('navbar-toggle');
      if (navMenu && navToggle) {
        navMenu.classList.remove('active');
        navToggle.classList.remove('active');
      }
    }
  });

  /**
   * Performance optimizations
   */
  
  // Lazy load images
  function initializeLazyLoading() {
    const images = document.querySelectorAll('img[data-src]');
    
    if ('IntersectionObserver' in window) {
      const imageObserver = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
          if (entry.isIntersecting) {
            const img = entry.target;
            img.src = img.dataset.src;
            img.classList.remove('lazy');
            imageObserver.unobserve(img);
          }
        });
      });
      
      images.forEach(img => imageObserver.observe(img));
    } else {
      // Fallback for older browsers
      images.forEach(img => {
        img.src = img.dataset.src;
        img.classList.remove('lazy');
      });
    }
  }
  
  // Initialize lazy loading
  document.addEventListener('DOMContentLoaded', initializeLazyLoading);

  /**
   * Remove line numbers from code blocks
   */
  function removeLineNumbers() {
    // Remove line number elements
    const lineNumbers = document.querySelectorAll('.line-numbers-rows, .rouge-gutter, .lineno, .rouge-table');
    lineNumbers.forEach(el => el.remove());
    
    // Remove line-numbers class from pre elements
    const preElements = document.querySelectorAll('pre.line-numbers, .highlight.line-numbers');
    preElements.forEach(el => el.classList.remove('line-numbers'));
    
    // Clean up Rouge table structure
    const rougeTables = document.querySelectorAll('.rouge-table');
    rougeTables.forEach(table => {
      const code = table.querySelector('.rouge-code');
      if (code && table.parentNode) {
        table.parentNode.replaceChild(code, table);
      }
    });
    
    // Ensure clean code structure
    const codeBlocks = document.querySelectorAll('pre code');
    codeBlocks.forEach(code => {
      // Remove any line number artifacts
      const lineNumbers = code.querySelectorAll('.line-numbers-rows, .rouge-gutter, .lineno');
      lineNumbers.forEach(el => el.remove());
    });
  }

})();