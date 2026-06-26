Feature: Claim Submission and Payment
  Scenario: A provider submits an Awakened-tier incident claim
    Given an active adventurer created by the scenario
    When the provider submits an 837 claim to the Adventure Society
    Then the claim is received with status "Submitted"
    And a 837 transaction is returned

  Scenario: The Society pays out a claim and sends remittance
    Given an approved claim created by the scenario
    When the Society processes payment
    Then an 835 remittance advice is generated
    And the claim status is updated to "Paid"

  Scenario: A provider checks the status of a pending claim
    Given a claim created by the scenario
    When the provider sends a claim status inquiry
    Then a 276 transaction is dispatched
    And a 277 response is returned with the current claim status

