# Medication Recommendation System


![banner.jpg](imgs/banner.jpg)


This project is a web application developed in Go that recommends medications based on specific pathologies. The application fetches data from a MySQL database containing information about medications and their usages. It also integrates Generative AI to enhance the recommendation process.

This project is designed to import medication data related to specific pathologies from the OpenFDA API into a MySQL database. It is written in Go and allows for flexible pathology lists via a configuration file.

🚨 **Attention:** This project is just a demonstration of what can be done with MySQL and AI. Its purpose is purely educational and for demonstration purposes only. It is in no way an application that will diagnose or treat any medical condition, nor provide medication recommendations


## Features

- Fetches medications based on user-defined pathologies.
- Integrates Generative AI to propose recommendations intelligently.
- Simple HTTP API to interact with.
- Easy integration with a MySQL database.

## Requirements

- Go (version 1.16 or newer)
- MySQL server
- Install Ollama on your computer: [download & install](https://ollama.com/download)

Once Ollama is installed, we'll use this model: qwen2.5:0.5b

To load the model, simply run the following command: ollama pull qwen2.5:0.5b (or ollama run qwen2.5:0.5b to interact directly with the CLI)

## Introduction : Generative AI

Generative AI refers to a set of artificial intelligence techniques capable of generating original content, including text, images, audio, and code. Based on advanced models such as Generative Adversarial Networks (GANs) and transformers (GPT, LLaMA, etc.), this technology enables content automation, decision support, and enhanced human-machine interaction.

With its adaptability and learning capabilities, Generative AI has a wide range of applications, from automated text generation and image synthesis to virtual assistants and digital art creation.

One example of a Generative AI application is chatbots that can answer questions, such as ClaudeAI, Gemini, and ChatGPT. The models used in these applications are often highly complex and require significant computational resources to operate.

> You can absolutely run LLMs on your own computer, and contrary to popular belief, you don’t necessarily need a high-end machine with a powerful GPU. There are lighter models that can run on standard laptops or even on a Raspberry Pi.

To do this, you need to choose a model that matches your needs and available resources, and use software that allows you to run it on your device.

Ollama is a lightweight framework designed to run large language models (LLMs) efficiently on local devices. It provides an easy way to download, manage, and execute AI models without requiring cloud-based processing. Ollama optimizes models for performance, allowing them to run on standard laptops and even low-power devices like Raspberry Pi.

With a simple command-line interface and built-in model support, Ollama makes it easy for developers to experiment with LLMs locally while maintaining control over their data and resources.

We're going to use **Ollama**.

### Ollama

Ollama is a lightweight framework designed to run large language models (LLMs) efficiently on local devices. It provides an easy way to download, manage, and execute AI models without requiring cloud-based processing. Ollama optimizes models for performance, allowing them to run on standard laptops and even low-power devices like Raspberry Pi.

With a simple command-line interface and built-in model support, Ollama makes it easy for developers to experiment with LLMs locally while maintaining control over their data and resources.



---

## ✅ Ressources

[OpenFDA](https://open.fda.gov/)

[Ollama](https://ollama.com/)
