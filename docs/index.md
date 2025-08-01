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
    KubeElasti <b>saves cost</b> using scale-to-zero <b>without losing any traffic</b>, requires <b>no code changes</b>, and integrates with your existing Kubernetes infrastructure.</p>
    <div class="hero-buttons">
      <a href="/src/gs-setup/" class="md-button md-button--primary">Get Started</a>
      <a href="https://discord.gg/qFyN73htgE" class="md-button">Join our community</a>
    </div>
  </div>
  <div class="hero-image">
    <img src="images/hero.png" alt="Illustration of KubeElasti scale-to-zero lifecycle">
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
      <div class="feature-icon">⚡</div>
      <h3>Zero Downtime</h3>
      <p>Queues requests during scale-up to ensure no traffic is lost</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔧</div>
      <h3>Simple Configuration</h3>
      <p>Easy setup with a single CRD and minimal configuration required</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🔄</div>
      <h3>Seamless Compatibility</h3>
      <p>Works with your existing Kubernetes setup, HPA, and Keda</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">📈</div>
      <h3>Out of Box Monitoring</h3>
      <p>Built-in monitoring with Prometheus metrics and Grafana dashboards</p>
    </div>
    <div class="feature-card">
      <div class="feature-icon">🛡️</div>
      <h3>Request Preservation</h3>
      <p>Ensures all incoming requests are processed even during scale operations</p>
    </div>
  </div>
</div>

<!-- How It Works Section -->
<div class="how-it-works-section">
  <h2>How It Works</h2>
  <div class="how-it-works-content">
    <div class="how-it-works-image">
      <img src="images/modes.png" alt="Diagram illustrating KubeElasti proxy and serve modes">
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
  <h2>Serverless with just 1 File</h2>
  <div class="get-started-steps">
    <div class="code-block">
      <pre><code> 
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
      <a href="/src/gs-setup/" class="md-button md-button--primary">Full Installation Guide</a>
    </div>
</div>

<div class="get-started-section">
  <h2>Demo - See KubeElasti in action!</h2>
<div style="position: relative; padding-bottom: 55.026178010471206%; height: 0;"><iframe src="https://www.loom.com/embed/53b7b524b4c342f99ba44fd5d8104265?sid=c88660d1-a569-470c-8224-b1fffde9a2c6" frameborder="0" webkitallowfullscreen mozallowfullscreen allowfullscreen style="position: absolute; top: 0; left: 0; width: 100%; height: 100%;"></iframe></div>
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
    <a href="https://discord.gg/qFyN73htgE" class="community-link">
      <span class="community-icon">💬</span>
      <span class="community-text">Discord</span>
    </a>
    <a href="https://github.com/truefoundry/KubeElasti/issues" class="community-link">
      <span class="community-icon">🆘</span>
      <span class="community-text">Report Issues</span>
    </a>
  </div>
</div>

<div class="footer-cta">
  <h2>Ready to optimize your Kubernetes resources?</h2>
  <a href="/src/gs-setup/" class="md-button md-button--primary">Get Started with KubeElasti</a>
</div>
