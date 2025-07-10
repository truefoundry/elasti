---
hide:
  - navigation
  - toc
---




<!-- Hero Section -->
<div class="hero-section">
  <div class="hero-content">
  <div class="hero-logo-title">
    <!-- <img src="images/logo/logo_theme_200x200.png" alt="KubeElasti Logo" class="hero-logo"> -->
     <h1 class="hero-title">KubeElasti</h1>
    </div>
    <p class="hero-subtitle">Serverless for Kubernetes</p>
    <p class="hero-description">Automatically scale your services to zero when idle and scale up when traffic arrives.
    <br><br>
    KubeElasti <b>saves cost</b> using scale-to-zero <b>without losing any requests</b>, requires <b>no code changes</b>, and integrates with your existing Kubernetes infrastructure.</p>
    <div class="hero-buttons">
      <a href="getting-started/" class="md-button md-button--primary">Get Started</a>
      <a href="https://github.com/truefoundry/KubeElasti" class="md-button">GitHub</a>
    </div>
  </div>
  <div class="hero-image">
    <img src="images/hero.png" alt="KubeElasti Modes">
  </div>
</div>

<!-- Companies Section -->
<!-- <div class="companies-section">
  <h2>Trusted By</h2>
  <div class="company-logos">
    <div class="company-logo placeholder">
      <img src="images/companies/truefoundry.png" alt="Truefoundry Logo">
    </div>
  </div>
</div> -->

<!-- Features Section -->
<div class="features-section">
  <h2>Key Features</h2>
  <div class="features-grid">
    <div class="feature-card">
      <div class="feature-icon">💰</div>
      <h3>Cost Optimization</h3>
      <p>Scale to zero when there's no traffic to save resources and reduce costs</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔄</div>
      <h3>Seamless Integration</h3>
      <p>Works with your existing Kubernetes setup, HPA, and Keda</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">⚡</div>
      <h3>Zero Downtime</h3>
      <p>Queues requests during scale-up to ensure no traffic is lost</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">📈</div>
      <h3>Prometheus Metrics</h3>
      <p>Built-in monitoring with Prometheus metrics and Grafana dashboards</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔌</div>
      <h3>Service Compatibility</h3>
      <p>Works with any Kubernetes service regardless of ingress or service mesh</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🚀</div>
      <h3>Deployment Support</h3>
      <p>Supports both standard Deployments and Argo Rollouts</p>
    </div>
  </div>
</div>

<!-- How It Works Section -->
<div class="how-it-works-section">
  <h2>How It Works</h2>
  <div class="how-it-works-content">
    <div class="how-it-works-image">
      <img src="images/modes.png" alt="KubeElasti Modes">
    </div>
    <div class="how-it-works-steps">
      <div class="step">
        <div class="step-number">1</div>
        <div class="step-content">
          <h3>Scaling Down</h3>
          <p>When all triggers indicate inactivity, KubeElasti scales the service to 0 replicas and switches to proxy mode</p>
        </div>
      </div>
      <div class="step">
        <div class="step-number">2</div>
        <div class="step-content">
          <h3>Traffic Queueing</h3>
          <p>In proxy mode, KubeElasti intercepts and queues incoming requests to the scaled-down service</p>
        </div>
      </div>
      <div class="step">
        <div class="step-number">3</div>
        <div class="step-content">
          <h3>Scaling Up</h3>
          <p>When traffic arrives, KubeElasti immediately scales the service back up to its minimum replicas</p>
        </div>
      </div>
      <div class="step">
        <div class="step-number">4</div>
        <div class="step-content">
          <h3>Serve Mode</h3>
          <p>Once the service is up, KubeElasti switches to serve mode and processes all queued requests</p>
        </div>
      </div>
    </div>
  </div>
</div>

<!-- Get Started Section -->
<div class="get-started-section">
  <h2>Get Started with 2 Commands</h2>
  
     
  <div class="get-started-steps">
    <div class="code-block">
      <pre><code> <span class="gray"># Install KubeElasti</span>
helm install <span class="green">elasti</span> oci://tfy.jfrog.io/tfy-helm/elasti --namespace <span class="green">elasti</span> --create-namespace

<span class="gray"># Create ElastiService CRD for the service you want to optimize</span>
<span class="gray"># Replace values between <> with actual values</span>
kubectl apply -f - &lt;&lt;EOF <span class="blue">
apiVersion: <span class="green">elasti.truefoundry.com/v1alpha1</span>
kind: <span class="green">ElastiService</span>
metadata:
  name: <span class="yellow">&lt;TARGET_SERVICE&gt;</span>
  namespace: <span class="yellow">&lt;TARGET_SERVICE_NAMESPACE&gt;</span>
spec:
  minTargetReplicas: <span class="green">1</span>
  service: <span class="yellow">&lt;TARGET_SERVICE_NAME&gt;</span>
  cooldownPeriod: <span class="green">5</span>
  scaleTargetRef:
    apiVersion: <span class="green">apps/v1</span>
    kind: <span class="green">deployments</span>
    name: <span class="yellow">&lt;TARGET_DEPLOYMENT_NAME&gt;</span>
  triggers:
    - type:   <span class="green">prometheus</span>
      metadata:
        <span class="gray"># Select a trigger metric to monitor</span>
        query: <span class="green">sum(rate(nginx_ingress_controller_nginx_process_requests_total[1m])) or vector(0)</span>
        <span class="gray"># Replace with the address of your Prometheus server</span>
        serverAddress: <span class="green">http://kube-prometheus-stack-prometheus.monitoring.svc.cluster.local:9090</span>
        threshold: <span class="green">"0.5"</span>
</span>EOF

<span class="gray"># 🎉 That's it! You just created a scale-to-zero service</span>
</code></pre>
    </div>
  </div>
       <div class="get-started-content">
      <p>KubeElasti is easy to set up and configure. Follow our step-by-step guide to get started.</p>
      <a href="getting-started/" class="md-button md-button--primary">Full Installation Guide</a>
    </div>
</div>

<!-- Community Section -->
<div class="community-section">
  <h2>Join Our Community</h2>
  <p>Get help, share your experience, and contribute to KubeElasti</p>
  <div class="community-links">
    <a href="https://github.com/truefoundry/KubeElasti" class="community-link">
      <span class="community-icon">🐱</span>
      <span class="community-text">GitHub</span>
    </a>
    <a href="https://github.com/truefoundry/KubeElasti/issues" class="community-link">
      <span class="community-icon">🆘</span>
      <span class="community-text">Report Issues</span>
    </a>
    <a href="https://github.com/truefoundry/KubeElasti/discussions" class="community-link">
      <span class="community-icon">💬</span>
      <span class="community-text">Discussions</span>
    </a>
  </div>
</div>

<div class="footer-cta">
  <h2>Ready to optimize your Kubernetes resources?</h2>
  <a href="getting-started/" class="md-button md-button--primary">Get Started with KubeElasti</a>
</div>


