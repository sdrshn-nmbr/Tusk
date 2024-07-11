document.addEventListener("DOMContentLoaded", () => {
  const fileList = document.getElementById("file-list");
  const aiAssistantBtn = document.getElementById("aiAssistantBtn");
  const aiAssistantModal = document.getElementById("aiAssistantModal");
  const closeAiAssistantModal = document.getElementById(
    "closeAiAssistantModal"
  );

  // Dummy data for file list
  const files = [
    { name: "Document.pdf", size: "10 MB", modified: "2 days ago" },
    { name: "Presentation.pptx", size: "25 MB", modified: "1 week ago" },
    { name: "Report.docx", size: "5 MB", modified: "3 months ago" },
  ];

  // Render file list
  function renderFileList() {
    fileList.innerHTML = files
      .map(
        (file) => `
            <div class="flex items-center justify-between p-4 hover:bg-muted/50">
                <div class="flex items-center space-x-4">
                    <i class="fas fa-file-alt text-primary text-2xl"></i>
                    <div>
                        <p class="font-medium">${file.name}</p>
                        <p class="text-sm text-muted-foreground">${file.size} • Last modified ${file.modified}</p>
                    </div>
                </div>
                <div class="flex space-x-2">
                    <button class="p-2 text-muted-foreground hover:text-foreground">
                        <i class="fas fa-eye"></i>
                    </button>
                    <button class="p-2 text-muted-foreground hover:text-foreground">
                        <i class="fas fa-download"></i>
                    </button>
                    <button class="p-2 text-muted-foreground hover:text-foreground">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </div>
        `
      )
      .join("");
  }

  // Initialize file list
  renderFileList();

  // AI Assistant Modal
  aiAssistantBtn.addEventListener("click", () => {
    aiAssistantModal.classList.remove("hidden");
  });

  closeAiAssistantModal.addEventListener("click", () => {
    aiAssistantModal.classList.add("hidden");
  });

  // Close modal when clicking outside
  aiAssistantModal.addEventListener("click", (e) => {
    if (e.target === aiAssistantModal) {
      aiAssistantModal.classList.add("hidden");
    }
  });

  // Add any additional functionality here (e.g., file upload, delete, search)
});
