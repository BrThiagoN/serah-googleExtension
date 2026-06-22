// content.js - Script de Conteúdo
// Este script é executado no contexto das páginas web que correspondem aos padrões definidos no manifest.json.
// Ele tem acesso total ao DOM da página, mas roda em um ambiente isolado (não compartilha variáveis com o JS da página).

console.log("Minha Extensão: Script de conteúdo (content.js) injetado com sucesso!");

// Exemplo: Ouvir mensagens vindas do popup.js ou background.js
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  console.log("Mensagem recebida no content.js:", message);

  if (message.action === "obterTextoSelecionado") {
    // Retorna a seleção de texto atual na página
    const selecao = window.getSelection().toString().trim();
    sendResponse({ status: "sucesso", texto: selecao });
  }

  // Retorna true se precisar responder de forma assíncrona
  return false;
});

// Espaço para você criar sua própria lógica de raspagem de dados (scraping),
// modificação de elementos da página ou injeção de novos componentes na interface:
// -----------------------------------------------------------------------------
// [Insira seu código personalizado aqui]
// -----------------------------------------------------------------------------
