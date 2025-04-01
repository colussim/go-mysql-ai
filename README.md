# MySQL & GenAI: Drug Recommendation with Vector Search


![banner.jpg](imgs/banner.jpg)


This project is a demonstration of how MySQL and its vector field can be leveraged to explore the potential of Generative AI (GenAI) in generating drug recommendations based on specific pathologies.

## Objective

The goal of this demonstration is to import drug data from the OpenFDA API and use an embedding model of type Qwen2.5:0.5b. These embeddings are then submitted to Ollama, which performs a vector search in MySQL to return the most relevant drugs based on the queried pathologies.

🚨 **Attention:** This project is just a demonstration of what can be done with MySQL and AI. Its purpose is purely educational and for demonstration purposes only. It is in no way an application that will diagnose or treat any medical condition, nor provide medication recommendations


## Features


✅ **Data Import:** A Go program extracts drug information related to various pathologies from the OpenFDA API.

✅ **Embedding Generation:** The Qwen2.5:0.5b model is used to generate vector representations of pathologies.

✅ **Storage in MySQL:** Data is stored in a MySQL database with a vector field for optimized search.

✅ **Vector Search with Ollama:** Queries MySQL to retrieve the most relevant drugs based on the generated embeddings.

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

Ollama lets you interact directly with an LLM via a command line (CLI) or REST API. I am interested in using only the REST API for this demonstration.

![ollama.png](imgs/ollama.png)


## Installation & Usage

I will not detail the installation of MySQL here; we assume that you have a functional instance.

**1. Install Ollama**

First, install Ollama by following the official installation guide  [here](https://ollama.com/download).

Since we are not using a GPU ✊, we will opt for lighter models that can run on standard laptops. We will install the *Qwen2.5:0.5b* model, but other models can also be used, such as:

- **Llama2-7B** : The lightest version of Llama2, but still heavier than Qwen2.5:0.5b.

Quantized versions of *Llama2-7B* : These are compressed versions that are lighter and faster while maintaining good performance.

Other Lightweight Model Options

- **Mistral 7B** : A recent model that offers good performance for its size.

- **TinyLlama** : A very lightweight version (1.1B parameters) based on Llama.

- **BLOOM-560m** : A multilingual model with 560 million parameters.

In general, larger models provide better performance but require more resources.
If your resources allow it, I recommend testing Llama2-7B.

To load the *Qwen2.5:0.5b* model , simply run the following command: 

 
```bash
ollama pull qwen2.5:0.5b

```

**1. Create the Database and Tables**

Next, we need to create a MySQL database and the necessary tables to store pathologies and recommended medications, along with their embeddings.

Database Schema :

```sql
-- Ceated Tablespace
CREATE TABLESPACE health_ts 
ADD DATAFILE '/Volumes/DATA/db/health/health_datafile.ibd' 
ENGINE=INNODB;

-- Created database
CREATE DATABASE health;

CREATE TABLE pathologies (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) UNIQUE,
    embedding VECTOR(896) 
) TABLESPACE health_ts;


CREATE TABLE medicationv (
    id INT AUTO_INCREMENT PRIMARY KEY,
    pathologie_id INT,
    drug_name VARCHAR(255),
    inactive_ingredient TEXT,
    purpose TEXT,
    keep_out_of_reach_of_children TEXT,
    warnings TEXT,
    spl_product_data_elements TEXT,
    dosage_and_administration TEXT,
    pregnancy_or_breast_feeding TEXT,
    package_label_principal_display_panel TEXT,
    indications_and_usage TEXT,
    embedding VECTOR(896),  
    CONSTRAINT fk_pathologie FOREIGN KEY (pathologie_id) REFERENCES pathologies(id)
) TABLESPACE health_ts;


```

**🧠 Embedding Structure**

Pathology Embedding Format:

- "For this pathology, Description: ..., Symptoms: ..., Treatments: ..."

Medication Embedding Format:

- "For this medication, Indications: ..., Purpose: ..., Dosage: ..., Warning: ..., Package Label: ..."





 It is written in Go and allows for flexible pathology lists via a configuration file.



---

## ✅ Ressources

[OpenFDA](https://open.fda.gov/)

[Ollama](https://ollama.com/)

[Ollama Embedding models](https://ollama.com/blog/embedding-models)

[Ollama API exemple](https://github.com/ollama/ollama/tree/main/api/examples)

