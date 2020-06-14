---
layout: default
title: Home
nav_order: 1
description: "Welcome to InfiniCache project site"
permalink: /
---

# InfiniCache
{: .fs-9 }

<!-- Welcome to InfiniCache porject site.
{: .fs-6 .fw-300 }
 -->
InfiniCache is a **cost-effective** memory cache that is built atop **ephemeral serverless functions**
{: .fs-6 .fw-300 }

[&nbsp;PAPER&nbsp;](https://www.usenix.org/system/files/fast20-wang_ao.pdf){: .btn .btn-primary .fs-5 .mb-4 .mb-md-0 .mr-6 } 
[&nbsp;TALK&nbsp;](https://www.youtube.com/watch?v=3_NmYAh5zek&t){: .btn .fs-5 .mb-4 .mb-md-0 .mr-6 } 
[&nbsp;CODE&nbsp;](https://github.com/mason-leap-lab/infinicache){: .btn .fs-5 .mb-4 .mb-md-0 .mr-6 }
[&nbsp;SLIDE&nbsp;](https://www.usenix.org/sites/default/files/conference/protected-files/fast20_slides_wang-ao.pdf){: .btn .fs-5 .mb-4 .mb-md-0 .mr-6 }
[POSTER](docs/fast20-infinicache_poster.pdf){: .btn .fs-5 .mb-4 .mb-md-0 }

### Press:
{: .fs-6 .fw-300 }

* IEEE Spectrum: [Cloud Services Tool Lets You Pay for Data You Use—Not Data You Store](https://spectrum.ieee.org/tech-talk/computing/networks/pay-cloud-services-data-tool-news)
* Mikhail Shilkov's detailed paper review: [InfiniCache: Distributed Cache on Top of AWS Lambda (paper review)](https://mikhail.io/2020/03/infinicache-distributed-cache-on-aws-lambda/)

---

## What is InfiniCache
{: .fs-6.5 }

InfiniCache is a first-of-its-kind in-memory object caching system that is completely built and deployed atop ephemeral serverless functions. InfiniCache exploits and orchestrates serverless functions’ memory resources to enable elastic pay per-use caching. InfiniCache's design combines erasure coding, intelligent billed duration control, and an efficient data backup mechanism to maximize data availability and cost effectiveness while balancing the risk of losing cached state and performance.

---
## Feature
{: .fs-6.5 }

- The very first in-memory object caching system powered by ephemeral and “stateless” cloud functions.
- Leverage several intelligent techniques to maxmize data availability, such as erasure coding periodic warm-up and periodic delta-sync techniques.
- InfiniCache achieves performance comparable to ElastiCache (600GB cache capacity) for large objects and improves the cost effectiveness of cloud IMOCs by 31 – 96x.

<style type="text/css">
.tg  {border-collapse:collapse;border-color:#ccc;border-spacing:0;}
.tg td{background-color:#fff;border-color:#ccc;border-style:solid;border-width:1px;color:#333;
  font-family:Arial, sans-serif;font-size:14px;overflow:hidden;padding:10px 5px;word-break:normal;}
.tg th{background-color:#f0f0f0;border-color:#ccc;border-style:solid;border-width:1px;color:#333;
  font-family:Arial, sans-serif;font-size:14px;font-weight:normal;overflow:hidden;padding:10px 5px;word-break:normal;}
.tg .tg-c6m9{font-family:"Comic Sans MS", cursive, sans-serif !important;;font-size:22px;font-weight:bold;text-align:center;
  vertical-align:top}
.tg .tg-a3by{background-color:#e7f2fe;color:#000000;font-family:Arial, Helvetica, sans-serif !important;;font-size:16px;
  font-style:italic;font-weight:bold;text-align:center;vertical-align:top}
.tg .tg-480y{font-family:Arial, Helvetica, sans-serif !important;;font-size:16px;text-align:center;vertical-align:top}
</style>
<table class="tg">
<thead>
  <tr>
    <th class="tg-c6m9">Service</th>
    <th class="tg-c6m9">Performance</th>
    <th class="tg-c6m9">Price</th>
  </tr>
</thead>
<tbody>
  <tr>
    <td class="tg-a3by">InfiniCache</td>
    <td class="tg-a3by">Fast</td>
    <td class="tg-a3by">Low</td>
  </tr>
  <tr>
    <td class="tg-480y"><a href="https://aws.amazon.com/elasticache/" target="_blank" rel="noopener noreferrer">ElastiCache</a></td>
    <td class="tg-480y">Fast</td>
    <td class="tg-480y">High</td>
  </tr>
  <tr>
    <td class="tg-480y"><a href="https://aws.amazon.com/s3" target="_blank" rel="noopener noreferrer">S3 Object Store</a></td>
    <td class="tg-480y">Slow</td>
    <td class="tg-480y">Low</td>
  </tr>
</tbody>
</table>


---
### Contact
Please feel free to reach out us if you have any questions about *InfiniCache*.

### Contributing

Please feel free to hack on InfiniCache! We're happy to accept contributions.

#### **Thank you to the contributors of InfiniCache!**
<!-- 
<ul class="list-style-none">
{% for contributor in site.github.contributors %}
  <li class="d-inline-block mr-1">
     <a href="{{ contributor.html_url }}"><img src="{{ contributor.avatar_url }}" width="32" height="32" alt="{{ contributor.login }}"/></a>
  </li>
{% endfor %}
</ul>
 -->