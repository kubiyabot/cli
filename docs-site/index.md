---
layout: default
title: Kubiya CLI - Your Agentic Automation Companion
description: Powerful command-line interface for managing Kubiya sources, serverless agents, and tools. Automate your engineering workflows with AI-powered agents.
---

<div class="hero">
    <div class="container">
        <h1>🤖 Kubiya CLI</h1>
        <p>Your Agentic Automation Companion</p>
        <div class="hero-actions">
            <a href="{{ '/pages/installation' | relative_url }}" class="btn btn-primary btn-lg">
                <i class="fas fa-download"></i> Install Now
            </a>
            <a href="{{ '/pages/quickstart' | relative_url }}" class="btn btn-outline-primary btn-lg">
                <i class="fas fa-rocket"></i> Quick Start
            </a>
            <a href="{{ '/pages/examples' | relative_url }}" class="btn btn-outline-primary btn-lg">
                <i class="fas fa-code"></i> Examples
            </a>
        </div>
    </div>
</div>

<div class="container">
    <section class="features-section">
        <h2 class="text-center mb-3">Powerful Features</h2>
        <div class="feature-grid">
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-robot"></i>
                </div>
                <h3>Serverless Agents</h3>
                <p>Deploy AI-powered serverless agents on your infrastructure. Create, manage, and scale agents with ease using Kubernetes and other orchestration platforms.</p>
            </div>
            
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-sync-alt"></i>
                </div>
                <h3>Workflow Execution</h3>
                <p>Execute workflows from local files, GitHub repositories, or URLs. Support for GitHub authentication, real-time tracking, and policy validation.</p>
            </div>
            
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-folder-open"></i>
                </div>
                <h3>Source Management</h3>
                <p>Scan and manage Git repositories and local directories. Add sources with version control, interactive browsing, and dynamic configurations.</p>
            </div>
            
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-tools"></i>
                </div>
                <h3>Tool Management</h3>
                <p>Execute tools with arguments, flags, and real-time feedback. Support for Docker containers, custom environments, and long-running operations.</p>
            </div>
            
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-lock"></i>
                </div>
                <h3>Secret Management</h3>
                <p>Securely store and manage secrets with role-based access control. Integrate with agents and tools for secure automation.</p>
            </div>
            
            <div class="feature-card">
                <div class="feature-icon">
                    <i class="fas fa-brain"></i>
                </div>
                <h3>MCP Integration</h3>
                <p>Model Context Protocol integration for Claude Desktop and Cursor IDE. Bridge local AI tools with Kubiya's powerful automation capabilities.</p>
            </div>
        </div>
    </section>
    
    <section class="install-section">
        <h2 class="text-center mb-3">Quick Installation</h2>
        
        <div class="install-tabs">
            <div class="install-tab active" data-tab="linux-mac">Linux/macOS</div>
            <div class="install-tab" data-tab="windows">Windows</div>
            <div class="install-tab" data-tab="apt">APT (Debian/Ubuntu)</div>
            <div class="install-tab" data-tab="build">Build from Source</div>
        </div>
        
        <div id="linux-mac" class="install-content active">
            <pre><code class="language-bash"># One-line installation
curl -fsSL https://cli.kubiya.ai/install.sh | bash</code></pre>
        </div>
        
        <div id="windows" class="install-content">
            <pre><code class="language-powershell"># PowerShell installation
iwr -useb https://cli.kubiya.ai/install.ps1 | iex</code></pre>
        </div>
        
        <div id="apt" class="install-content">
            <pre><code class="language-bash"># Add repository
curl -fsSL https://cli.kubiya.ai/apt-key.gpg | sudo gpg --dearmor -o /usr/share/keyrings/kubiya-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/kubiya-archive-keyring.gpg] https://cli.kubiya.ai/apt stable main" | sudo tee /etc/apt/sources.list.d/kubiya.list

# Install
sudo apt update && sudo apt install kubiya-cli</code></pre>
        </div>
        
        <div id="build" class="install-content">
            <pre><code class="language-bash"># Clone and build
git clone https://github.com/kubiyabot/cli.git
cd cli
make build && make install</code></pre>
        </div>
    </section>
    
    <section class="getting-started">
        <h2 class="text-center mb-3">Get Started in Minutes</h2>
        
        <div class="example-grid">
            <div class="example-card">
                <h3>1. Configure Authentication</h3>
                <pre><code class="language-bash">export KUBIYA_API_KEY="your-api-key"
kubiya config init</code></pre>
            </div>
            
            <div class="example-card">
                <h3>2. Create Your First Agent</h3>
                <pre><code class="language-bash">kubiya agent create \
  --name "DevOps Agent" \
  --desc "Handles DevOps automation" \
  --interactive</code></pre>
            </div>
            
            <div class="example-card">
                <h3>3. Execute a Workflow</h3>
                <pre><code class="language-bash">kubiya workflow execute \
  myorg/deploy-scripts \
  --var env=production</code></pre>
            </div>
            
            <div class="example-card">
                <h3>4. Chat with Your Agent</h3>
                <pre><code class="language-bash">kubiya chat \
  --interactive \
  -m "Deploy the latest version"</code></pre>
            </div>
        </div>
    </section>
    
    <section class="use-cases">
        <h2 class="text-center mb-3">Use Cases</h2>
        
        <div class="feature-grid">
            <div class="feature-card">
                <h3>🚀 CI/CD Automation</h3>
                <p>Automate your deployment pipelines with intelligent agents that can handle complex workflows, rollbacks, and monitoring.</p>
                <a href="{{ '/pages/examples' | relative_url }}#cicd" class="btn btn-outline">View Examples</a>
            </div>
            
            <div class="feature-card">
                <h3>☸️ Kubernetes Management</h3>
                <p>Deploy and manage Kubernetes resources with agents that understand your infrastructure and can make intelligent decisions.</p>
                <a href="{{ '/pages/examples' | relative_url }}#kubernetes" class="btn btn-outline">View Examples</a>
            </div>
            
            <div class="feature-card">
                <h3>🔒 Security Operations</h3>
                <p>Automate security scans, compliance checks, and incident response with AI-powered agents that understand your security policies.</p>
                <a href="{{ '/pages/examples' | relative_url }}#security" class="btn btn-outline">View Examples</a>
            </div>
            
            <div class="feature-card">
                <h3>📊 Monitoring & Alerting</h3>
                <p>Create intelligent monitoring agents that can analyze metrics, identify issues, and take corrective actions automatically.</p>
                <a href="{{ '/pages/examples' | relative_url }}#monitoring" class="btn btn-outline">View Examples</a>
            </div>
        </div>
    </section>
</div>