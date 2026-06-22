// popup.js - Lógica da Interface da Popup/Painel Lateral
// Este script lida com a interação do usuário na interface e se comunica com background.js ou content.js.
// Sempre use async/await para chamadas assíncronas do Chrome Extension API.

document.addEventListener("DOMContentLoaded", async () => {
  console.log("Popup carregada!");

  // Elementos do DOM
  const textareaSelecionado = document.getElementById("texto-selecionado");
  const btnCapturar = document.getElementById("btn-capturar");
  const inputPergunta = document.getElementById("input-pergunta");
  const btnEnviar = document.getElementById("btn-enviar");
  const btnLimpar = document.getElementById("btn-limpar");
  const ultimoSalvoSpan = document.getElementById("ultimo-salvo");
  const linkOptions = document.getElementById("link-options");

  // Elementos do Card da IA
  const aiStatusRow = document.getElementById("ai-status-row");
  const aiVerdictBadge = document.getElementById("ai-verdict-badge");
  const aiScoreValue = document.getElementById("ai-score-value");
  const aiExplanationText = document.getElementById("ai-explanation-text");
  const aiSourcesSection = document.getElementById("ai-sources-section");
  const aiSourcesList = document.getElementById("ai-sources-list");

  // Elementos de Feedback no DOM
  const feedbackLog = document.getElementById("feedback-log");
  const feedbackIcon = feedbackLog.querySelector(".feedback-icon");
  const feedbackMsg = feedbackLog.querySelector(".feedback-message");
  let feedbackTimeout;

  // Função para exibir logs/alertas simples e explicativos na interface
  function mostrarFeedback(mensagem, tipo, codigo) {
    clearTimeout(feedbackTimeout);
    feedbackLog.className = `feedback-banner ${tipo}`;
    feedbackIcon.textContent = tipo === "success" ? "✅" : "❌";
    feedbackMsg.innerHTML = `<strong>[${codigo}]</strong> ${mensagem}`;
    
    // Oculta o alerta automaticamente após 4 segundos (ou 5 segundos se for erro)
    const timeoutDuration = tipo === "error" ? 5000 : 4000;
    feedbackTimeout = setTimeout(() => {
      feedbackLog.classList.add("hidden");
    }, timeoutDuration);
  }

  // Carregar valor previamente salvo no armazenamento local
  try {
    let valorSalvo = null;
    if (typeof chrome !== "undefined" && chrome.storage && chrome.storage.local) {
      const data = await chrome.storage.local.get("usuarioDado");
      valorSalvo = data.usuarioDado;
    } else {
      valorSalvo = localStorage.getItem("usuarioDado");
    }

    if (valorSalvo) {
      ultimoSalvoSpan.textContent = valorSalvo;
      console.log("Histórico carregado com sucesso. Código: [STG_100]");
    }
  } catch (error) {
    console.error("Erro ao carregar dado do storage:", error);
    mostrarFeedback("Não foi possível restaurar o último salvo.", "error", "STG_502");
  }

  // Função para obter o texto selecionado (grifado) na aba ativa
  async function capturarTextoSelecionado(silencioso = false) {
    try {
      if (typeof chrome === "undefined" || !chrome.tabs) {
        throw new Error("API chrome.tabs não disponível.");
      }
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      if (!tab) {
        textareaSelecionado.value = "";
        textareaSelecionado.placeholder = "Nenhuma aba ativa encontrada.";
        if (!silencioso) {
          mostrarFeedback("Nenhuma aba ativa encontrada para captura.", "error", "CAP_404");
        }
        return false;
      }

      // Envia mensagem para o content.js da página ativa
      const resposta = await chrome.tabs.sendMessage(tab.id, { action: "obterTextoSelecionado" });
      if (resposta && resposta.status === "sucesso") {
        if (resposta.texto) {
          textareaSelecionado.value = resposta.texto;
          if (!silencioso) {
            mostrarFeedback("Texto selecionado capturado com sucesso!", "success", "CAP_200");
          }
          return true;
        } else {
          textareaSelecionado.value = "";
          textareaSelecionado.placeholder = "Nenhum texto selecionado na página. Grife um texto no site e clique em capturar.";
          if (!silencioso) {
            mostrarFeedback("Nenhum texto está grifado/selecionado na página.", "error", "CAP_204");
          }
          return false;
        }
      } else {
        textareaSelecionado.value = "";
        textareaSelecionado.placeholder = "Não foi possível obter a seleção desta página (recarregue a página para tentar novamente).";
        if (!silencioso) {
          mostrarFeedback("Não foi possível conectar ao script da página. Recarregue a aba.", "error", "CAP_501");
        }
        return false;
      }
    } catch (error) {
      console.error("Erro ao capturar texto:", error);
      textareaSelecionado.value = "";
      textareaSelecionado.placeholder = "Não permitido em alguns sites";
      if (!silencioso) {
        mostrarFeedback("Captura não permitida pelo navegador nesta página específica.", "error", "CAP_403");
      }
      return false;
    }
  }

  // Captura o texto silenciosamente ao abrir o painel lateral
  await capturarTextoSelecionado(true);

  // Função auxiliar para enviar e salvar a pergunta no storage
  async function enviarPergunta(valor) {
    if (!valor) {
      mostrarFeedback("O campo de pergunta está vazio. Digite algo primeiro.", "error", "VAL_400");
      return;
    }
    try {
      if (typeof chrome !== "undefined" && chrome.storage && chrome.storage.local) {
        await chrome.storage.local.set({ usuarioDado: valor });
      } else {
        localStorage.setItem("usuarioDado", valor);
      }
      ultimoSalvoSpan.textContent = valor;
      console.log("Dado enviado e salvo com sucesso:", valor);
      mostrarFeedback("Pergunta enviada para análise!", "success", "STG_201");
      
      // Dispara a análise da IA com base no texto enviado
      await chamarAPIAnalise(valor);
    } catch (error) {
      console.error("Erro ao salvar no storage:", error);
      mostrarFeedback(`Erro ao processar o envio: ${error.message}`, "error", "STG_501");
    }
  }

  // AÇÃO: Capturar Seleção manualmente ao clicar no botão, colar no textarea, copiar para clipboard, colar no input e enviar automaticamente
  btnCapturar.addEventListener("click", async () => {
    const sucesso = await capturarTextoSelecionado(false);
    if (sucesso) {
      const texto = textareaSelecionado.value.trim();
      if (texto) {
        try {
          // Copiar para a área de transferência (Clipboard)
          try {
            await navigator.clipboard.writeText(texto);
            console.log("Texto copiado para a área de transferência.");
          } catch (clipErr) {
            console.warn("Não foi possível copiar para a área de transferência:", clipErr);
          }

          // Apaga o texto que já estiver no input antes de colar
          inputPergunta.value = "";
          
          // Cola o texto capturado no input
          inputPergunta.value = texto;

          // Envia o texto automaticamente
          await enviarPergunta(texto);
        } catch (error) {
          console.error("Erro ao processar captura:", error);
          mostrarFeedback("Falha ao salvar a seleção capturada no histórico.", "error", "STG_503");
        }
      } else {
        mostrarFeedback("O texto selecionado está vazio.", "error", "CAP_400");
      }
    }
  });

  // 3. AÇÃO: Enviar manualmente o texto digitado no input
  btnEnviar.addEventListener("click", async () => {
    const valor = inputPergunta.value.trim();
    await enviarPergunta(valor);
  });

  // AÇÃO: Enviar pergunta ao pressionar a tecla Enter no input
  inputPergunta.addEventListener("keydown", async (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      const valor = inputPergunta.value.trim();
      await enviarPergunta(valor);
    }
  });

  // AÇÃO: Limpar Sessão
  btnLimpar.addEventListener("click", async () => {
    try {
      if (typeof chrome !== "undefined" && chrome.storage && chrome.storage.local) {
        await chrome.storage.local.clear();
      } else {
        localStorage.clear();
      }
      ultimoSalvoSpan.textContent = "-";
      inputPergunta.value = "";
      textareaSelecionado.value = "";
      textareaSelecionado.placeholder = "Selecione um texto em qualquer site e abra o popup, ou clique abaixo...";
      
      // Resetar estado do Card da IA
      aiStatusRow.classList.add("hidden");
      aiSourcesSection.classList.add("hidden");
      aiVerdictBadge.classList.remove("verdade", "falso", "duvidoso");
      aiVerdictBadge.textContent = "-";
      aiScoreValue.textContent = "-";
      aiScoreValue.style.color = "";
      aiExplanationText.value = "";
      aiSourcesList.innerHTML = "";
      
      mostrarFeedback("Sessão limpa com sucesso!", "success", "STG_202");
    } catch (error) {
      console.error("Erro ao limpar sessão:", error);
      mostrarFeedback("Falha ao limpar a sessão no storage.", "error", "STG_504");
    }
  });

  // 4. AÇÃO: Abrir Página de Opções (se houver) ou painel de configurações
  linkOptions.addEventListener("click", (e) => {
    e.preventDefault();
    if (chrome.runtime.openOptionsPage) {
      chrome.runtime.openOptionsPage();
      mostrarFeedback("Abrindo as opções da extensão...", "success", "CFG_200");
    } else {
      mostrarFeedback("Página de configurações não configurada no manifest.", "error", "CFG_404");
    }
  });

  // Função para chamar o backend para análise de IA
  async function chamarAPIAnalise(texto) {
    // 1. Mostrar estado de carregamento no próprio campo explicativo
    aiStatusRow.classList.add("hidden");
    aiSourcesSection.classList.add("hidden");
    aiVerdictBadge.classList.remove("verdade", "falso", "duvidoso");
    aiVerdictBadge.textContent = "-";
    aiScoreValue.textContent = "-";
    aiExplanationText.value = "Analisando texto... Por favor, aguarde alguns segundos.";
    
    try {
      // Chamada real para a rota definida no COMUNICATION.md
      let response;
      try {
        response = await fetch("http://localhost:3000/api/verify", {
          method: "POST",
          headers: {
            "Content-Type": "application/json"
          },
          body: JSON.stringify({ text: texto })
        });
      } catch (networkErr) {
        console.error("Erro de conexão com o servidor:", networkErr);
        aiStatusRow.classList.add("hidden");
        aiSourcesSection.classList.add("hidden");
        aiVerdictBadge.textContent = "-";
        aiScoreValue.textContent = "-";
        aiExplanationText.value = "Erro: Sem conexão com o backend local de verificação.";
        mostrarFeedback("Sem conexão com o backend de análise da IA na porta 3000.", "error", "AI_503");
        return;
      }
      
      if (!response.ok) {
        let errJson;
        try {
          errJson = await response.json();
        } catch (parseErr) {
          // Response is not JSON
        }
        
        aiStatusRow.classList.add("hidden");
        aiSourcesSection.classList.add("hidden");
        aiVerdictBadge.textContent = "-";
        aiScoreValue.textContent = "-";
        
        if (errJson && errJson.status === "error") {
          console.error(`[API ERROR] ${errJson.code}: ${errJson.message}`);
          aiExplanationText.value = `Erro no servidor: ${errJson.message}`;
          mostrarFeedback(errJson.message, "error", errJson.code);
        } else {
          console.error(`[HTTP ERROR] Status: ${response.status}`);
          aiExplanationText.value = `Erro no servidor (Status: ${response.status}).`;
          mostrarFeedback(`Erro do servidor (Status ${response.status}).`, "error", `HTTP_${response.status}`);
        }
        return;
      }
      
      const resJson = await response.json();
      
      if (!resJson || !resJson.analysis) {
        throw new Error("Resposta em formato inválido pelo servidor.");
      }

      const dadosResponse = {
        verdict: resJson.analysis.verdict.toLowerCase(),
        score: Math.round(resJson.analysis.reliability_score * 100),
        explanation: resJson.analysis.explanation,
        sources: resJson.analysis.sources || []
      };
      
      // 3. Atualizar a UI
      aiStatusRow.classList.remove("hidden");
      
      // Limpar classes de veredicto anteriores
      aiVerdictBadge.classList.remove("verdade", "falso", "duvidoso");
      
      // Definir novo estado
      aiVerdictBadge.classList.add(dadosResponse.verdict);
      if (dadosResponse.verdict === "verdade" || dadosResponse.verdict === "confiavel") {
        aiVerdictBadge.textContent = "Confiável";
        aiScoreValue.style.color = "#10b981";
      } else if (dadosResponse.verdict === "falso") {
        aiVerdictBadge.textContent = "Falso";
        aiScoreValue.style.color = "#ef4444";
      } else {
        aiVerdictBadge.textContent = "Duvidoso";
        aiScoreValue.style.color = "#f59e0b";
      }
      
      aiScoreValue.textContent = `${dadosResponse.score}%`;
      aiExplanationText.value = dadosResponse.explanation;
      
      // Adicionar fontes
      if (dadosResponse.sources && dadosResponse.sources.length > 0) {
        aiSourcesList.innerHTML = "";
        dadosResponse.sources.forEach(src => {
          const li = document.createElement("li");
          const a = document.createElement("a");
          a.href = src.url;
          a.target = "_blank";
          a.textContent = src.title;
          li.appendChild(a);
          aiSourcesList.appendChild(li);
        });
        aiSourcesSection.classList.remove("hidden");
      } else {
        aiSourcesSection.classList.add("hidden");
      }
      
    } catch (err) {
      console.error("Erro inesperado ao processar análise da IA:", err);
      aiStatusRow.classList.add("hidden");
      aiSourcesSection.classList.add("hidden");
      aiExplanationText.value = "Erro inesperado ao processar resposta do servidor.";
      mostrarFeedback("Erro inesperado ao processar resposta do servidor.", "error", "SYS_500");
    }
  }
});
